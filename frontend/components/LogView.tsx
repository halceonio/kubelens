
import React, { useState, useEffect, useRef, useMemo, memo } from 'react';
import { LogEntry, Pod, LogLevel, AppResource } from '../types';
import { getPodLogs } from '../services/k8sService';
import * as ReactWindow from 'react-window';
import { MOCK_CONFIG } from '../constants';

// Robustly resolve FixedSizeList from the ESM module wrapper
const FixedSizeList = (ReactWindow as any).FixedSizeList || (ReactWindow as any).default?.FixedSizeList || (ReactWindow as any).default;

interface LogViewProps {
  resource: Pod | AppResource;
  onClose: () => void;
  isMaximized?: boolean;
  accessToken?: string | null;
}

type TimeFilterType = 'all' | '1m' | '5m' | '15m' | '30m' | '1h';

interface RowData {
  logs: LogEntry[];
  terminatedPods: string[];
  selectedIndices: Set<number>;
  onRowClick: (index: number, event: React.MouseEvent) => void;
  showTimestamp: boolean;
  isApp: boolean;
  isWrapping: boolean;
}

const deriveLevel = (message: string): LogLevel => {
  const lower = message.toLowerCase();
  if (lower.includes('error') || lower.includes('failed') || lower.includes('exception')) return 'ERROR';
  if (lower.includes('warn')) return 'WARNING';
  if (lower.includes('debug')) return 'DEBUG';
  return 'INFO';
};

const parseSSEEvent = (raw: string): LogEntry | null => {
  const lines = raw.split('\n');
  let id = '';
  const dataLines: string[] = [];
  lines.forEach(line => {
    if (line.startsWith('id:')) {
      id = line.slice(3).trim();
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart());
    }
  });

  if (dataLines.length === 0) return null;
  try {
    const payload = JSON.parse(dataLines.join('\n')) as Partial<LogEntry> & {
      message?: string;
      timestamp?: string;
      podName?: string;
      containerName?: string;
      level?: LogLevel;
      id?: string;
    };
    const timestamp = payload.timestamp || id || new Date().toISOString();
    const message = payload.message || '';
    return {
      id: payload.id || `${timestamp}-${payload.podName || 'pod'}`,
      timestamp,
      message,
      podName: payload.podName || 'unknown',
      containerName: payload.containerName || 'main',
      level: payload.level || deriveLevel(message)
    };
  } catch {
    return null;
  }
};

const LogRow = memo(({ index, style, data }: { index: number; style: React.CSSProperties; data: RowData }) => {
  const { logs, terminatedPods, selectedIndices, onRowClick, showTimestamp, isApp, isWrapping } = data;
  const log = logs[index];
  
  if (!log) return <div style={style} />;
  
  const isTerminated = terminatedPods.includes(log.podName);
  const isSelected = selectedIndices.has(index);
  
  return (
    <div 
      style={style} 
      onClick={(e) => onRowClick(index, e)}
      className={`flex gap-3 md:gap-4 group hover:bg-white/5 px-2 items-start py-1 mono text-[10px] md:text-[11px] leading-tight overflow-hidden border-b border-white/[0.03] cursor-pointer select-none transition-colors ${
        isSelected ? 'bg-sky-500/30 border-sky-500/50' : isTerminated ? 'opacity-40' : ''
      }`}
    >
      {showTimestamp && (
        <span className={`shrink-0 select-none w-16 md:w-20 pt-0.5 ${isTerminated ? 'text-slate-600' : 'text-slate-500'}`}>
          [{new Date(log.timestamp).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })}]
        </span>
      )}
      
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
          {log.level}
        </span>
      </div>
      
      <span className={`pt-0.5 ${isWrapping ? 'whitespace-normal break-all' : 'truncate'} ${isTerminated ? 'text-slate-500 italic' : 'text-slate-300'}`}>
        {log.message}
      </span>
    </div>
  );
});

const LogView: React.FC<LogViewProps> = ({ resource, onClose, isMaximized, accessToken }) => {
  const isApp = 'type' in resource;
  const initialPods = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
  
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [filter, setFilter] = useState<string>('');
  const [selectedLevel, setSelectedLevel] = useState<LogLevel | 'ALL'>('ALL');
  const [selectedPods, setSelectedPods] = useState<string[]>(initialPods);
  const [timeFilter, setTimeFilter] = useState<TimeFilterType>('all');
  const [isPodDropdownOpen, setIsPodDropdownOpen] = useState(false);
  
  const [isAutoScroll, setIsAutoScroll] = useState(true);
  const [isWrapping, setIsWrapping] = useState(false);
  const [showTimestamp, setShowTimestamp] = useState(true);

  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(new Set());
  const [lastSelectedIndex, setLastSelectedIndex] = useState<number | null>(null);
  
  const listRef = useRef<any>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
  const containerRef = useRef<HTMLDivElement>(null);

  const enterpriseMetadata = useMemo(() => {
    const prefix = MOCK_CONFIG.label_prefix + '/';
    const metadata: Record<string, string> = {};
    const sourceData = { ...(resource.labels || {}), ...(resource.annotations || {}) };
    
    Object.entries(sourceData).forEach(([key, value]) => {
      if (key.startsWith(prefix)) {
        const strippedKey = key.substring(prefix.length);
        metadata[strippedKey] = value as string;
      }
    });
    return metadata;
  }, [resource.labels, resource.annotations]);

  const terminatedPods = useMemo(() => {
    return initialPods.filter(p => p.includes('terminated'));
  }, [initialPods]);

  const lastTimestampRef = useRef<string | null>(null);
  const streamAbortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    let mounted = true;

    const fetchLogs = async () => {
      const podNames = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
      const containers = !isApp ? (resource as Pod).containers.map(c => c.name) : ['main-app'];
      const data = await getPodLogs(podNames[0], 500, containers, podNames);
      if (!mounted) return;
      setLogs(data);
    };

    if (!resource || !resource.name) return;

    if (!accessToken) {
      fetchLogs();
      const interval = setInterval(async () => {
        const podNames = isApp ? (resource as AppResource).podNames : [(resource as Pod).name];
        const containers = !isApp ? (resource as Pod).containers.map(c => c.name) : ['main-app'];
        const newLogs = await getPodLogs(podNames[0], 5, containers, podNames);
        if (!mounted) return;
        setLogs(prev => [...prev.slice(-1500), ...newLogs]);
      }, 4000);

      return () => {
        mounted = false;
        clearInterval(interval);
      };
    }

    const connectStream = async () => {
      const basePath = isApp
        ? `/api/v1/namespaces/${resource.namespace}/apps/${resource.name}/logs`
        : `/api/v1/namespaces/${resource.namespace}/pods/${resource.name}/logs`;
      const url = new URL(basePath, window.location.origin);
      url.searchParams.set('tail', '500');
      if (lastTimestampRef.current) {
        url.searchParams.set('since', lastTimestampRef.current);
      }

      const controller = new AbortController();
      streamAbortRef.current = controller;

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
              lastTimestampRef.current = parsed.timestamp;
              setLogs(prev => {
                const next = [...prev, parsed];
                return next.slice(-2000);
              });
            }
            splitIndex = buffer.indexOf('\n\n');
          }
        }
      } catch (err) {
        if (!mounted) return;
        await new Promise(resolve => setTimeout(resolve, 1500));
        if (mounted) {
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
  }, [resource.name, resource.namespace, isApp, accessToken]);

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
      if (!selectedPods.includes(log.podName)) return false;
      if (selectedLevel !== 'ALL' && log.level !== selectedLevel) return false;
      if (filter && !log.message.toLowerCase().includes(filter.toLowerCase())) return false;
      if (timeFilter !== 'all') {
        const logTime = new Date(log.timestamp).getTime();
        const diffMinutes = (now - logTime) / 1000 / 60;
        const limit = parseInt(timeFilter.replace('m', '').replace('h', '60'));
        if (diffMinutes > limit) return false;
      }
      return true;
    });
  }, [logs, selectedPods, selectedLevel, filter, timeFilter]);

  useEffect(() => {
    setSelectedIndices(new Set());
    setLastSelectedIndex(null);
  }, [filter, selectedLevel, selectedPods]);

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
    isApp,
    isWrapping
  };

  return (
    <div className={`flex flex-col h-full bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden shadow-lg dark:shadow-2xl relative transition-colors duration-200 ${isMaximized ? 'col-span-full' : ''}`}>
      <div className="bg-white dark:bg-slate-900 px-3 md:px-4 py-2 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between flex-wrap gap-2 md:gap-3 transition-colors duration-200">
        <div className="flex items-center gap-2 md:gap-3">
          <div className="flex items-center gap-1.5 md:gap-2 pr-2 md:pr-3 border-r border-slate-200 dark:border-slate-700">
            <span className="text-[10px] font-bold text-sky-600 dark:text-sky-500 px-1.5 py-0.5 bg-sky-500/10 rounded uppercase">{isApp ? 'App' : 'Pod'}</span>
            <h3 className="text-xs md:text-sm font-mono font-medium text-slate-700 dark:text-slate-200 truncate max-w-[100px] md:max-w-[140px]">{resource.name}</h3>
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
        {filteredLogs.length === 0 ? (
          <div className="h-full flex items-center justify-center text-slate-600 italic text-sm">
            No logs found matching filters
          </div>
        ) : (
          dimensions.height > 0 && dimensions.width > 0 && FixedSizeList && (
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

          <div className="text-[10px] text-slate-400 dark:text-slate-600 font-mono">
            {filteredLogs.length} entries
          </div>
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
              placeholder="Search..." 
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-400 focus:ring-1 focus:ring-sky-500/50 w-24 md:w-32 text-right transition-all"
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default LogView;
