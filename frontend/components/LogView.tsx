
import React, { useState, useEffect, useRef, useMemo, memo, useCallback } from 'react';
import { LogEntry, Pod, LogLevel, AppResource, UiConfig } from '../types';
import { getPodLogs } from '../services/k8sService';
import { FixedSizeList } from 'react-window';
import { DEFAULT_UI_CONFIG, USE_MOCKS } from '../constants';

interface LogViewProps {
  resource: Pod | AppResource;
  onClose: () => void;
  isMaximized?: boolean;
  accessToken?: string | null;
  config?: UiConfig | null;
  initialLogLevel?: LogLevel | 'ALL';
  onLogLevelChange?: (level: LogLevel | 'ALL') => void;
}

type TimeFilterType = 'all' | '1m' | '5m' | '15m' | '30m' | '1h';

interface RowData {
  logs: LogEntry[];
  terminatedPods: string[];
  selectedIndices: Set<number>;
  onRowClick: (index: number, event: React.MouseEvent) => void;
  showTimestamp: boolean;
  showDetails: boolean;
  isApp: boolean;
  isWrapping: boolean;
  searchQuery: string;
  activeMatchIndex: number | null;
}

type StreamStats = {
  dropped: number;
  buffered: number;
};

type ParsedEvent =
  | { kind: 'log'; entry: LogEntry }
  | { kind: 'marker'; entry: LogEntry }
  | { kind: 'stats'; stats: StreamStats }
  | { kind: 'heartbeat' };

const ANSI_REGEX = /\x1b\[[0-9;]*m/g;

const stripAnsi = (message: string) => message.replace(ANSI_REGEX, '');

const ANSI_COLORS = [
  '#000000',
  '#cc0000',
  '#4e9a06',
  '#c4a000',
  '#3465a4',
  '#75507b',
  '#06989a',
  '#d3d7cf'
];

const ANSI_BRIGHT_COLORS = [
  '#555753',
  '#ef2929',
  '#8ae234',
  '#fce94f',
  '#729fcf',
  '#ad7fa8',
  '#34e2e2',
  '#eeeeec'
];

type AnsiSegment = {
  text: string;
  style: React.CSSProperties;
};

const parseAnsiSegments = (input: string): AnsiSegment[] => {
  const segments: AnsiSegment[] = [];
  const regex = /\x1b\[([0-9;]*)m/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let style: React.CSSProperties = {};

  const pushText = (text: string) => {
    if (!text) return;
    segments.push({ text, style: { ...style } });
  };

  while ((match = regex.exec(input)) !== null) {
    if (match.index > lastIndex) {
      pushText(input.slice(lastIndex, match.index));
    }

    const codes = match[1]
      .split(';')
      .filter(Boolean)
      .map((val) => Number(val));

    if (codes.length === 0) {
      style = {};
    }

    for (const code of codes) {
      if (Number.isNaN(code)) continue;
      if (code === 0) {
        style = {};
      } else if (code === 1) {
        style.fontWeight = 600;
      } else if (code === 2) {
        style.opacity = 0.7;
      } else if (code === 3) {
        style.fontStyle = 'italic';
      } else if (code === 4) {
        style.textDecoration = 'underline';
      } else if (code === 22) {
        style.fontWeight = undefined;
        style.opacity = undefined;
      } else if (code === 23) {
        style.fontStyle = undefined;
      } else if (code === 24) {
        style.textDecoration = undefined;
      } else if (code === 39) {
        style.color = undefined;
      } else if (code === 49) {
        style.backgroundColor = undefined;
      } else if (code >= 30 && code <= 37) {
        style.color = ANSI_COLORS[code - 30];
      } else if (code >= 90 && code <= 97) {
        style.color = ANSI_BRIGHT_COLORS[code - 90];
      } else if (code >= 40 && code <= 47) {
        style.backgroundColor = ANSI_COLORS[code - 40];
      } else if (code >= 100 && code <= 107) {
        style.backgroundColor = ANSI_BRIGHT_COLORS[code - 100];
      }
    }

    lastIndex = regex.lastIndex;
  }

  if (lastIndex < input.length) {
    pushText(input.slice(lastIndex));
  }

  return segments;
};

const deriveLevel = (message: string): LogLevel => {
  const lower = stripAnsi(message).toLowerCase();
  if (lower.includes('error') || lower.includes('failed') || lower.includes('exception')) return 'ERROR';
  if (lower.includes('warn')) return 'WARNING';
  if (lower.includes('debug')) return 'DEBUG';
  return 'INFO';
};

const parseSSEEvent = (raw: string): ParsedEvent | null => {
  const lines = raw.split('\n');
  let id = '';
  let eventType = 'log';
  const dataLines: string[] = [];
  lines.forEach(line => {
    if (line.startsWith('id:')) {
      id = line.slice(3).trim();
    } else if (line.startsWith('event:')) {
      eventType = line.slice(6).trim();
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart());
    }
  });

  if (dataLines.length === 0) return null;
  try {
    const payload = JSON.parse(dataLines.join('\n')) as any;

    if (eventType === 'stats') {
      return {
        kind: 'stats',
        stats: {
          dropped: Number(payload?.dropped ?? 0),
          buffered: Number(payload?.buffered ?? 0)
        }
      };
    }

    if (eventType === 'heartbeat') {
      return { kind: 'heartbeat' };
    }

    const timestamp = payload?.timestamp || id || new Date().toISOString();
    const message = payload?.message || '';
    const baseEntry: LogEntry = {
      id: payload?.id || `${timestamp}-${payload?.podName || 'pod'}`,
      timestamp,
      message,
      podName: payload?.podName || 'unknown',
      containerName: payload?.containerName || 'main',
      level: payload?.level || deriveLevel(message)
    };

    if (eventType === 'marker') {
      return {
        kind: 'marker',
        entry: {
          ...baseEntry,
          kind: 'marker',
          markerKind: payload?.kind || 'marker'
        }
      };
    }

    return { kind: 'log', entry: { ...baseEntry, kind: 'log' } };
  } catch {
    return null;
  }
};

const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const highlightText = (text: string, query: string) => {
  if (!query) return text;
  const regex = new RegExp(escapeRegExp(query), 'ig');
  const parts = text.split(regex);
  const matches = text.match(regex);
  if (!matches) return text;
  const output: React.ReactNode[] = [];
  parts.forEach((part, idx) => {
    output.push(part);
    if (matches[idx]) {
      output.push(
        <mark key={`${part}-${idx}`} className="bg-sky-500/30 text-slate-100 rounded px-0.5">
          {matches[idx]}
        </mark>
      );
    }
  });
  return output;
};

const renderAnsiWithHighlight = (text: string, query: string) => {
  const segments = parseAnsiSegments(text);
  if (!query) {
    return segments.map((segment, idx) => (
      <span key={`ansi-${idx}`} style={segment.style}>
        {segment.text}
      </span>
    ));
  }
  const regex = new RegExp(escapeRegExp(query), 'ig');
  return segments.map((segment, idx) => {
    const matches = segment.text.match(regex);
    if (!matches) {
      return (
        <span key={`ansi-${idx}`} style={segment.style}>
          {segment.text}
        </span>
      );
    }
    const parts = segment.text.split(regex);
    const nodes: React.ReactNode[] = [];
    parts.forEach((part, partIdx) => {
      nodes.push(part);
      if (matches[partIdx]) {
        nodes.push(
          <mark key={`ansi-${idx}-mark-${partIdx}`} className="bg-sky-500/30 text-slate-100 rounded px-0.5">
            {matches[partIdx]}
          </mark>
        );
      }
    });
    return (
      <span key={`ansi-${idx}`} style={segment.style}>
        {nodes}
      </span>
    );
  });
};

const LogRow = memo(({ index, style, data }: { index: number; style: React.CSSProperties; data: RowData }) => {
  const { logs, terminatedPods, selectedIndices, onRowClick, showTimestamp, showDetails, isApp, isWrapping, searchQuery, activeMatchIndex } = data;
  const log = logs[index];
  
  if (!log) return <div style={style} />;
  
  const rowStyle = {
    ...style,
    width: 'max-content',
    minWidth: '100%'
  };

  const isTerminated = terminatedPods.includes(log.podName);
  const isSelected = selectedIndices.has(index);
  
  const isMarker = log.kind === 'marker';
  const isActiveMatch = activeMatchIndex === index;

  return (
    <div 
      style={rowStyle} 
      onClick={(e) => onRowClick(index, e)}
      className={`flex gap-3 md:gap-4 group hover:bg-white/5 px-2 items-start py-1 mono text-[10px] md:text-[11px] leading-tight border-b border-white/[0.03] cursor-pointer select-none transition-colors ${
        isActiveMatch ? 'bg-sky-500/20 border-sky-500/40' : isSelected ? 'bg-sky-500/30 border-sky-500/50' : isTerminated ? 'opacity-40' : ''
      }`}
    >
      {showTimestamp && (
        <span className={`shrink-0 select-none w-16 md:w-20 pt-0.5 ${isTerminated ? 'text-slate-600' : 'text-slate-500'}`}>
          [{new Date(log.timestamp).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })}]
        </span>
      )}
      
      {showDetails && (
        <div className="flex items-start gap-3 md:gap-4 shrink-0">
          {isApp && (
            <span 
              className={`font-bold select-none px-2 py-0.5 rounded w-24 md:w-32 lg:w-48 truncate text-left border shrink-0 ${
                isTerminated 
                ? 'text-slate-500 bg-slate-800/50 border-slate-700/50' 
                : 'text-sky-500 bg-sky-500/10 border-sky-500/20'
              }`} 
              title={log.podName + (isTerminated ? ' (Terminated)' : '')}
            >
              {log.podName}
            </span>
          )}
          <span className={`font-bold shrink-0 w-12 md:w-16 select-none pt-0.5 ${
            isTerminated 
              ? 'text-slate-600' 
              : log.level === 'ERROR' ? 'text-red-400' : log.level === 'WARNING' ? 'text-amber-400' : 'text-slate-400'
          }`}>
            {isMarker ? 'MARK' : log.level}
          </span>
        </div>
      )}
      
      <span className={`pt-0.5 ${isWrapping ? 'whitespace-normal break-all' : 'whitespace-nowrap'} ${isMarker ? 'text-sky-300 italic' : isTerminated ? 'text-slate-500 italic' : 'text-slate-300'}`}>
        {renderAnsiWithHighlight(log.message, searchQuery)}
      </span>
    </div>
  );
});

const LogView: React.FC<LogViewProps> = ({ resource, onClose, isMaximized, accessToken, config, initialLogLevel, onLogLevelChange }) => {
  const isApp = 'type' in resource;
  const initialPods = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
  const effectiveConfig = config ?? DEFAULT_UI_CONFIG;
  
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [filter, setFilter] = useState<string>('');
  const [selectedLevel, setSelectedLevel] = useState<LogLevel | 'ALL'>(initialLogLevel ?? 'ALL');
  const [selectedPods, setSelectedPods] = useState<string[]>(initialPods);
  const [timeFilter, setTimeFilter] = useState<TimeFilterType>('all');
  const [isPodDropdownOpen, setIsPodDropdownOpen] = useState(false);
  const [selectedContainer, setSelectedContainer] = useState<string>('');
  
  const [isAutoScroll, setIsAutoScroll] = useState(true);
  const [isWrapping, setIsWrapping] = useState(false);
  const [showTimestamp, setShowTimestamp] = useState(true);
  const [showDetails, setShowDetails] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [streamStatus, setStreamStatus] = useState<'connecting' | 'live' | 'reconnecting' | 'paused' | 'stale'>('connecting');
  const [isPaused, setIsPaused] = useState(false);
  const [droppedCount, setDroppedCount] = useState(0);
  const [bufferedCount, setBufferedCount] = useState(0);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeMatchOffset, setActiveMatchOffset] = useState(0);

  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(new Set());
  const [lastSelectedIndex, setLastSelectedIndex] = useState<number | null>(null);
  
  const listRef = useRef<any>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
  const containerRef = useRef<HTMLDivElement>(null);

  const enterpriseMetadata = useMemo(() => {
    const prefix = effectiveConfig.kubernetes.label_prefix ? `${effectiveConfig.kubernetes.label_prefix}/` : '';
    const metadata: Record<string, string> = {};
    const sourceData = { ...(resource.labels || {}), ...(resource.annotations || {}) };
    
    Object.entries(sourceData).forEach(([key, value]) => {
      if (prefix && key.startsWith(prefix)) {
        const strippedKey = key.substring(prefix.length);
        metadata[strippedKey] = value as string;
      }
    });
    return metadata;
  }, [resource.labels, resource.annotations]);

  const terminatedPods = useMemo(() => {
    return initialPods.filter(p => p.includes('terminated'));
  }, [initialPods]);

  const availableContainers = useMemo(() => {
    if (isApp) {
      return (resource as AppResource).containers?.map(c => c.name) ?? [];
    }
    return (resource as Pod).containers?.map(c => c.name) ?? [];
  }, [resource, isApp]);

  const lastTimestampRef = useRef<string | null>(null);
  const streamAbortRef = useRef<AbortController | null>(null);
  const lastEventAtRef = useRef<number | null>(null);
  const isPausedRef = useRef(false);

  useEffect(() => {
    if (availableContainers.length > 0) {
      setSelectedContainer(prev => prev || availableContainers[0]);
    } else {
      setSelectedContainer('');
    }
  }, [availableContainers]);

  useEffect(() => {
    setSelectedPods(initialPods);
  }, [resource.name, resource.namespace]);

  useEffect(() => {
    if (initialLogLevel) {
      setSelectedLevel(initialLogLevel);
    }
  }, [initialLogLevel, resource.name, resource.namespace]);

  useEffect(() => {
    if (onLogLevelChange) {
      onLogLevelChange(selectedLevel);
    }
  }, [selectedLevel, onLogLevelChange]);

  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (isPausedRef.current) return;
      if (!lastEventAtRef.current) return;
      const age = Date.now() - lastEventAtRef.current;
      if (age > 30000) {
        setStreamStatus('stale');
      }
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    let mounted = true;

    const fetchLogs = async () => {
      const podNames = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
      const containers = selectedContainer
        ? [selectedContainer]
        : !isApp
          ? (resource as Pod).containers.map(c => c.name)
          : ['main-app'];
      const data = await getPodLogs(podNames[0], 500, containers, podNames);
      if (!mounted) return;
      setLogs(data);
    };

    if (!resource || !resource.name) return;

    if (!accessToken && !USE_MOCKS) {
      setLogs([]);
      setLoadError('Log streaming requires authentication');
      return;
    }

    if (!accessToken) {
      setStreamStatus('live');
      setLoadError(null);
      fetchLogs().catch((err) => {
        if (!mounted) return;
        const message = err instanceof Error ? err.message : 'Failed to load logs';
        setLoadError(message);
      });
      const interval = setInterval(async () => {
        try {
          const podNames = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
          const containers = selectedContainer
            ? [selectedContainer]
            : !isApp
              ? (resource as Pod).containers.map(c => c.name)
              : ['main-app'];
          const newLogs = await getPodLogs(podNames[0], 5, containers, podNames);
          if (!mounted) return;
          setLogs(prev => [...prev.slice(-1500), ...newLogs]);
        } catch (err) {
          if (!mounted) return;
          const message = err instanceof Error ? err.message : 'Failed to load logs';
          setLoadError(message);
        }
      }, 4000);

      return () => {
        mounted = false;
        clearInterval(interval);
      };
    }

    if (isPaused) {
      setStreamStatus('paused');
      if (streamAbortRef.current) {
        streamAbortRef.current.abort();
      }
      return;
    }

    const connectStream = async () => {
      setStreamStatus('connecting');
      const basePath = isApp
        ? `/api/v1/namespaces/${resource.namespace}/apps/${resource.name}/logs`
        : `/api/v1/namespaces/${resource.namespace}/pods/${resource.name}/logs`;
      const url = new URL(basePath, window.location.origin);
      url.searchParams.set('tail', '500');
      if (selectedContainer) {
        url.searchParams.set('container', selectedContainer);
      }
      if (lastTimestampRef.current) {
        url.searchParams.set('since', lastTimestampRef.current);
      }

      const controller = new AbortController();
      streamAbortRef.current = controller;

      let received = false;
      try {
        const res = await fetch(url.toString(), {
          headers: {
            Authorization: `Bearer ${accessToken}`
          },
          signal: controller.signal
        });
        if (!res.ok || !res.body) {
          throw new Error(`stream error ${res.status}`);
        }
        setLoadError(null);

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (mounted) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          let splitIndex = buffer.indexOf('\n\n');
          while (splitIndex !== -1) {
            const rawEvent = buffer.slice(0, splitIndex);
            buffer = buffer.slice(splitIndex + 2);
            const parsed = parseSSEEvent(rawEvent);
            if (parsed) {
              if (!received) {
                received = true;
                setLoadError(null);
              }
              lastEventAtRef.current = Date.now();
              setStreamStatus('live');
              if (parsed.kind === 'log' || parsed.kind === 'marker') {
                const entry = parsed.entry;
                lastTimestampRef.current = entry.timestamp;
                setLogs(prev => {
                  const next = [...prev, entry];
                  return next.slice(-2000);
                });
              } else if (parsed.kind === 'stats') {
                setDroppedCount(parsed.stats.dropped);
                setBufferedCount(parsed.stats.buffered);
              }
            }
            splitIndex = buffer.indexOf('\n\n');
          }
        }
      } catch (err) {
        if (!mounted) return;
        if (isPausedRef.current) {
          return;
        }
        const message = err instanceof Error ? err.message : 'Log stream disconnected';
        setLoadError(message);
        setStreamStatus('reconnecting');
        await new Promise(resolve => setTimeout(resolve, 1500));
        if (mounted && !isPausedRef.current) {
          connectStream();
        }
      }
    };

    connectStream();

    return () => {
      mounted = false;
      if (streamAbortRef.current) {
        streamAbortRef.current.abort();
      }
    };
  }, [resource.name, resource.namespace, isApp, accessToken, selectedContainer, isPaused]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsPodDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        setDimensions({
          width: containerRef.current.offsetWidth,
          height: containerRef.current.offsetHeight
        });
      }
    };
    updateDimensions();
    const resizeObserver = new ResizeObserver(updateDimensions);
    if (containerRef.current) resizeObserver.observe(containerRef.current);
    return () => resizeObserver.disconnect();
  }, [isMaximized]);

  const filteredLogs = useMemo(() => {
    const now = new Date().getTime();
    return logs.filter(log => {
      if (log.kind !== 'marker') {
        if (!selectedPods.includes(log.podName)) return false;
        if (selectedLevel !== 'ALL' && log.level !== selectedLevel) return false;
      }
      const message = stripAnsi(log.message);
      if (filter && !message.toLowerCase().includes(filter.toLowerCase())) return false;
      if (timeFilter !== 'all') {
        const logTime = new Date(log.timestamp).getTime();
        const diffMinutes = (now - logTime) / 1000 / 60;
        const limit = parseInt(timeFilter.replace('m', '').replace('h', '60'));
        if (diffMinutes > limit) return false;
      }
      return true;
    });
  }, [logs, selectedPods, selectedLevel, filter, timeFilter]);

  const matchIndices = useMemo(() => {
    if (!searchQuery) return [];
    const query = searchQuery.toLowerCase();
    const indices: number[] = [];
    filteredLogs.forEach((log, idx) => {
      if (stripAnsi(log.message).toLowerCase().includes(query)) {
        indices.push(idx);
      }
    });
    return indices;
  }, [filteredLogs, searchQuery]);

  useEffect(() => {
    setActiveMatchOffset(0);
  }, [searchQuery, filteredLogs]);

  const activeMatchIndex = matchIndices.length > 0 ? matchIndices[Math.min(activeMatchOffset, matchIndices.length - 1)] : null;

  const jumpToMatch = useCallback((direction: number) => {
    if (matchIndices.length === 0) return;
    setActiveMatchOffset(prev => {
      const next = (prev + direction + matchIndices.length) % matchIndices.length;
      const targetIndex = matchIndices[next];
      if (listRef.current) {
        listRef.current.scrollToItem(targetIndex, 'center');
      }
      return next;
    });
  }, [matchIndices]);

  useEffect(() => {
    setSelectedIndices(new Set());
    setLastSelectedIndex(null);
  }, [filter, selectedLevel, selectedPods, showDetails, showTimestamp]);

  useEffect(() => {
    if (isAutoScroll && listRef.current && filteredLogs.length > 0) {
      listRef.current.scrollToItem(filteredLogs.length - 1, 'end');
    }
  }, [filteredLogs.length, isAutoScroll]);

  const handleRowClick = (index: number, event: React.MouseEvent) => {
    const newSelected = new Set(selectedIndices);
    
    if (event.shiftKey && lastSelectedIndex !== null) {
      const start = Math.min(lastSelectedIndex, index);
      const end = Math.max(lastSelectedIndex, index);
      for (let i = start; i <= end; i++) {
        newSelected.add(i);
      }
    } else {
      if (newSelected.has(index)) {
        newSelected.delete(index);
      } else {
        newSelected.add(index);
      }
      setLastSelectedIndex(index);
    }
    
    setSelectedIndices(newSelected);
  };

  const copySelectedLogs = () => {
    const selectedLines = Array.from(selectedIndices)
      .sort((a, b) => a - b)
      .map(index => {
        const log = filteredLogs[index];
        const ts = showTimestamp ? `[${new Date(log.timestamp).toLocaleTimeString()}] ` : '';
        const pod = isApp ? `[${log.podName}] ` : '';
        return `${ts}${pod}[${log.level}] ${log.message}`;
      })
      .join('\n');
    
    navigator.clipboard.writeText(selectedLines);
    alert(`Copied ${selectedIndices.size} lines to clipboard`);
  };

  const togglePod = (name: string) => {
    setSelectedPods(prev => 
      prev.includes(name) ? prev.filter(n => n !== name) : [...prev, name]
    );
  };


  const rowData: RowData = {
    logs: filteredLogs,
    terminatedPods,
    selectedIndices,
    onRowClick: handleRowClick,
    showTimestamp,
    showDetails,
    isApp,
    isWrapping,
    searchQuery,
    activeMatchIndex
  };

  const streamMeta = (() => {
    switch (streamStatus) {
      case 'live':
        return { label: 'Live', color: 'bg-emerald-500', text: 'text-emerald-500' };
      case 'reconnecting':
        return { label: 'Reconnecting', color: 'bg-amber-500', text: 'text-amber-500' };
      case 'paused':
        return { label: 'Paused', color: 'bg-slate-500', text: 'text-slate-400' };
      case 'stale':
        return { label: 'Idle', color: 'bg-slate-500', text: 'text-slate-400' };
      default:
        return { label: 'Connecting', color: 'bg-sky-500', text: 'text-sky-500' };
    }
  })();

  const handleTogglePause = useCallback(() => {
    setIsPaused(prev => {
      const next = !prev;
      setStreamStatus(next ? 'paused' : 'connecting');
      return next;
    });
  }, []);

  const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.code !== 'Space') return;
    const target = event.target as HTMLElement;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) {
      return;
    }
    event.preventDefault();
    handleTogglePause();
  }, [handleTogglePause]);

  return (
    <div
      className={`flex flex-col h-full bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden shadow-lg dark:shadow-2xl relative transition-colors duration-200 ${isMaximized ? 'col-span-full' : ''}`}
      tabIndex={0}
      onKeyDown={handleKeyDown}
      onClick={(event) => (event.currentTarget as HTMLDivElement).focus()}
    >
      <div className="bg-white dark:bg-slate-900 px-3 md:px-4 py-2 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between flex-wrap gap-2 md:gap-3 transition-colors duration-200">
        <div className="flex items-center gap-2 md:gap-3">
          <div className="flex items-center gap-1.5 md:gap-2 pr-2 md:pr-3 border-r border-slate-200 dark:border-slate-700">
            <span className="text-[10px] font-bold text-sky-600 dark:text-sky-500 px-1.5 py-0.5 bg-sky-500/10 rounded uppercase">{isApp ? 'App' : 'Pod'}</span>
            <h3 className="text-xs md:text-sm font-mono font-medium text-slate-700 dark:text-slate-200 truncate max-w-[100px] md:max-w-[140px]">{resource.name}</h3>
          </div>

          <div className="flex items-center gap-1.5 px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700">
            <span className={`w-1.5 h-1.5 rounded-full ${streamMeta.color}`}></span>
            <span className={`text-[9px] font-bold uppercase tracking-wide ${streamMeta.text}`}>{streamMeta.label}</span>
          </div>
          
          {isApp && (
            <div className="relative" ref={dropdownRef}>
              <button 
                onClick={() => setIsPodDropdownOpen(!isPodDropdownOpen)}
                className="flex items-center gap-1.5 md:gap-2 px-1.5 py-1 rounded bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[9px] md:text-[10px] font-bold text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white transition-all"
              >
                <svg className="w-3 md:w-3.5 h-3 md:h-3.5 text-sky-600 dark:text-sky-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M4 6h16M4 12h16m-7 6h7" strokeWidth={2.5} /></svg>
                <span className="hidden sm:inline">{selectedPods.length} Sources</span>
                <span className="sm:hidden">{selectedPods.length}</span>
                <svg className={`w-3 h-3 transition-transform ${isPodDropdownOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M19 9l-7 7-7-7" strokeWidth={2} /></svg>
              </button>
              
              {isPodDropdownOpen && (
                <div className="absolute top-full left-0 mt-1 w-64 md:w-72 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg shadow-xl dark:shadow-2xl z-[60] overflow-hidden transition-colors duration-200">
                  <div className="p-2 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 flex justify-between items-center px-3">
                    <span className="text-[9px] font-bold text-slate-400 dark:text-slate-500 uppercase">Pods</span>
                    <button 
                      onClick={() => setSelectedPods(isApp ? (resource as AppResource).podNames : [])}
                      className="text-[9px] text-sky-600 dark:text-sky-500 font-bold"
                    >
                      Select All
                    </button>
                  </div>
                  <div className="max-h-48 overflow-y-auto custom-scrollbar p-1">
                    {(resource as AppResource).podNames.map(p => {
                      const isTerminated = terminatedPods.includes(p);
                      return (
                        <button
                          key={p}
                          onClick={() => togglePod(p)}
                          className={`w-full flex items-center justify-between px-3 py-2 text-[11px] hover:bg-slate-100 dark:hover:bg-slate-700/50 rounded transition-colors ${isTerminated ? 'opacity-60' : ''}`}
                        >
                          <div className="flex items-center gap-2.5 overflow-hidden">
                            <div className={`w-3.5 h-3.5 rounded border flex items-center justify-center transition-colors ${selectedPods.includes(p) ? 'bg-sky-500 border-sky-500' : 'bg-slate-50 dark:bg-slate-900 border-slate-300 dark:border-slate-600'}`}>
                              {selectedPods.includes(p) && (
                                <svg className="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                                </svg>
                              )}
                            </div>
                            <span className={`truncate ${selectedPods.includes(p) ? 'text-slate-900 dark:text-white' : 'text-slate-500 dark:text-slate-400'}`}>{p}</span>
                          </div>
                          {isTerminated && <span className="text-[8px] bg-slate-200 dark:bg-slate-700 text-slate-500 dark:text-slate-400 px-1 rounded font-bold uppercase shrink-0">Terminated</span>}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}

          {availableContainers.length > 1 && (
            <select
              value={selectedContainer}
              onChange={(e) => setSelectedContainer(e.target.value)}
              className="bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[9px] md:text-[10px] font-bold text-slate-600 dark:text-slate-300 rounded px-2 py-1"
            >
              {availableContainers.map((container) => (
                <option key={container} value={container}>
                  {container}
                </option>
              ))}
            </select>
          )}
        </div>

        <div className="flex items-center gap-2">
          <div className="flex items-center bg-slate-100 dark:bg-slate-800 rounded-lg p-0.5 border border-slate-200 dark:border-slate-700 transition-colors duration-200">
            {['ALL', 'INFO', 'WARN', 'ERR'].map(level => (
              <button
                key={level}
                onClick={() => {
                  const mapping: Record<string, LogLevel | 'ALL'> = {
                    'ALL': 'ALL', 'INFO': 'INFO', 'WARN': 'WARNING', 'ERR': 'ERROR'
                  };
                  setSelectedLevel(mapping[level]);
                }}
                className={`px-1.5 md:px-2 py-1 text-[8px] md:text-[9px] font-bold rounded transition-all ${
                  (level === 'ALL' && selectedLevel === 'ALL') || 
                  (level === 'INFO' && selectedLevel === 'INFO') || 
                  (level === 'WARN' && selectedLevel === 'WARNING') || 
                  (level === 'ERR' && selectedLevel === 'ERROR')
                    ? 'bg-white dark:bg-slate-700 text-sky-600 dark:text-sky-400 shadow-sm' 
                    : 'text-slate-500 hover:text-slate-800 dark:hover:text-slate-300'
                }`}
              >
                {level}
              </button>
            ))}
          </div>

          <button
            onClick={handleTogglePause}
            className="p-1 md:p-1.5 text-slate-400 hover:text-sky-500 transition-colors"
            title={isPaused ? 'Resume stream (Space)' : 'Pause stream (Space)'}
          >
            {isPaused ? (
              <svg className="w-4 md:w-5 h-4 md:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 4l14 8-14 8V4z" />
              </svg>
            ) : (
              <svg className="w-4 md:w-5 h-4 md:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M10 4h4v16h-4zM4 4h4v16H4z" />
              </svg>
            )}
          </button>

          <button 
            onClick={onClose}
            className="p-1 md:p-1.5 text-slate-400 hover:text-red-500 transition-colors"
          >
            <svg className="w-4 md:w-5 h-4 md:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M6 18L18 6M6 6l12 12" strokeWidth={2.5}/></svg>
          </button>
        </div>
      </div>

      {Object.entries(enterpriseMetadata).length > 0 && (
        <div className="bg-slate-50 dark:bg-slate-900/70 px-4 py-1.5 border-b border-slate-200 dark:border-slate-800 flex items-center gap-3 flex-wrap overflow-x-auto no-scrollbar transition-colors duration-200">
          {Object.entries(enterpriseMetadata).map(([key, value]) => (
            <div key={key} className="flex items-center gap-1.5 px-2 py-0.5 rounded-md border border-sky-200 dark:border-sky-500/20 bg-sky-50 dark:bg-sky-500/5 shadow-sm">
              <span className="text-[9px] font-bold text-slate-500 dark:text-slate-500 uppercase tracking-tighter shrink-0">{key}</span>
              <div className="w-px h-2.5 bg-slate-200 dark:bg-slate-700"></div>
              <span className="text-[10px] font-bold text-sky-600 dark:text-sky-400 truncate max-w-[200px]" title={value}>{value}</span>
            </div>
          ))}
        </div>
      )}


      <div 
        ref={containerRef}
        className="flex-1 bg-slate-950 p-2 overflow-hidden"
      >
        {loadError && (
          <div className="mb-2 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-[10px] text-red-500">
            {loadError}
          </div>
        )}
        {filteredLogs.length === 0 ? (
          <div className="h-full flex items-center justify-center text-slate-600 italic text-sm">
            No logs found matching filters
          </div>
        ) : (
          dimensions.height > 0 && dimensions.width > 0 && FixedSizeList && (
            <div className="h-full w-full overflow-x-auto overflow-y-hidden">
              <FixedSizeList
                ref={listRef}
                height={dimensions.height - 16}
                itemCount={filteredLogs.length}
                itemSize={isWrapping ? 54 : 26}
                width={dimensions.width - 16}
                className="custom-scrollbar"
                itemData={rowData}
              >
                {LogRow}
              </FixedSizeList>
            </div>
          )
        )}
      </div>

      <div className="h-auto min-h-[36px] bg-white dark:bg-slate-900 border-t border-slate-200 dark:border-slate-800 px-4 py-1.5 flex flex-wrap items-center justify-between shrink-0 gap-y-2 transition-colors duration-200">
        <div className="flex items-center gap-3 md:gap-5">
          <label className="flex items-center gap-2 cursor-pointer group">
            <input 
              type="checkbox" 
              checked={isAutoScroll} 
              onChange={(e) => setIsAutoScroll(e.target.checked)}
              className="w-3 h-3 rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-sky-600 dark:text-sky-500 focus:ring-0 focus:ring-offset-0 transition-colors"
            />
            <span className="text-[10px] font-bold text-slate-500 uppercase group-hover:text-slate-700 dark:group-hover:text-slate-400 hidden sm:inline">Auto-scroll</span>
          </label>
          
          <label className="flex items-center gap-2 cursor-pointer group">
            <input 
              type="checkbox" 
              checked={isWrapping} 
              onChange={(e) => setIsWrapping(e.target.checked)}
              className="w-3 h-3 rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-sky-600 dark:text-sky-500 focus:ring-0 focus:ring-offset-0 transition-colors"
            />
            <span className="text-[10px] font-bold text-slate-500 uppercase group-hover:text-slate-700 dark:group-hover:text-slate-400 hidden sm:inline">Wrap</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer group">
            <input 
              type="checkbox" 
              checked={showTimestamp} 
              onChange={(e) => setShowTimestamp(e.target.checked)}
              className="w-3 h-3 rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-sky-600 dark:text-sky-500 focus:ring-0 focus:ring-offset-0 transition-colors"
            />
            <span className="text-[10px] font-bold text-slate-500 uppercase group-hover:text-slate-700 dark:group-hover:text-slate-400 hidden sm:inline">Time</span>
          </label>

          <label className="flex items-center gap-2 cursor-pointer group">
            <input
              type="checkbox"
              checked={showDetails}
              onChange={(e) => setShowDetails(e.target.checked)}
              className="w-3 h-3 rounded bg-white dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-sky-600 dark:text-sky-500 focus:ring-0 focus:ring-offset-0 transition-colors"
            />
            <span className="text-[10px] font-bold text-slate-500 uppercase group-hover:text-slate-700 dark:group-hover:text-slate-400 hidden sm:inline">Details</span>
          </label>

          <div className="text-[10px] text-slate-400 dark:text-slate-600 font-mono">
            {filteredLogs.length} entries
          </div>
          {(droppedCount > 0 || bufferedCount > 0) && (
            <div className="text-[10px] text-amber-500 font-mono">
              Dropped {droppedCount} · Buffer {bufferedCount}
            </div>
          )}
        </div>
        
        <div className="flex items-center gap-3">
          {selectedIndices.size > 0 && (
            <button
              onClick={copySelectedLogs}
              className="flex items-center gap-1.5 px-2 py-1 bg-sky-50 dark:bg-sky-500/10 hover:bg-sky-100 dark:hover:bg-sky-500/20 border border-sky-200 dark:border-sky-500/30 text-sky-600 dark:text-sky-400 text-[9px] font-bold uppercase rounded transition-all"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" strokeWidth={2}/></svg>
              Copy ({selectedIndices.size})
            </button>
          )}

          <div className="relative">
            <input 
              type="text" 
              placeholder="Filter..." 
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-400 focus:ring-1 focus:ring-sky-500/50 w-24 md:w-32 text-right transition-all"
            />
          </div>

          <div className="flex items-center gap-1.5">
            <input
              type="text"
              placeholder="Find..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-400 focus:ring-1 focus:ring-sky-500/50 w-20 md:w-28 text-right transition-all"
            />
            <button
              onClick={() => jumpToMatch(-1)}
              className="p-1 text-slate-400 hover:text-sky-500"
              title="Previous match"
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <button
              onClick={() => jumpToMatch(1)}
              className="p-1 text-slate-400 hover:text-sky-500"
              title="Next match"
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </button>
            {searchQuery && (
              <span className="text-[9px] text-slate-400">
                {matchIndices.length > 0 ? `${Math.min(activeMatchOffset + 1, matchIndices.length)}/${matchIndices.length}` : '0/0'}
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default LogView;
