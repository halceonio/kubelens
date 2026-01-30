
import React, { useState, useEffect, useCallback } from 'react';
import Sidebar from './components/Sidebar';
import LogView from './components/LogView';
import PodInspector from './components/PodInspector';
import AuthGuard from './components/AuthGuard';
import { Pod, AppResource, ResourceIdentifier, AuthUser, UiConfig, SavedView, ViewFilters, LogLevel, LogViewPreferences } from './types';
import { getPodByName, getAppByName } from './services/k8sService';
import { fetchSession, saveSession, clearSession, SessionPayload } from './services/sessionService';
import { fetchConfig } from './services/configService';
import { DEFAULT_UI_CONFIG, USE_MOCKS } from './constants';

const STORAGE_KEY_ACTIVE = 'kubelens_active_resources';
const STORAGE_KEY_PINNED = 'kubelens_pinned_resources';
const STORAGE_KEY_THEME = 'kubelens_theme';
const STORAGE_KEY_SIDEBAR = 'kubelens_sidebar_open';
const STORAGE_KEY_SAVED_VIEWS = 'kubelens_saved_views';
const STORAGE_KEY_VIEW_FILTERS = 'kubelens_view_filters';
const STORAGE_KEY_ACTIVE_VIEW = 'kubelens_active_view';
const STORAGE_KEY_LOG_VIEW = 'kubelens_log_view';

const getDefaultTheme = () => {
  const saved = localStorage.getItem(STORAGE_KEY_THEME);
  if (saved === 'light' || saved === 'dark') return saved;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
};

const App: React.FC = () => {
  const [activeResources, setActiveResources] = useState<(Pod | AppResource)[]>([]);
  const [pinnedResources, setPinnedResources] = useState<ResourceIdentifier[]>([]);
  const [inspectingResource, setInspectingResource] = useState<Pod | AppResource | null>(null);
  const [isRestoring, setIsRestoring] = useState(true);
  const [sessionToken, setSessionToken] = useState<string | null>(null);
  const [authUser, setAuthUser] = useState<AuthUser | null>(null);
  const [remoteSession, setRemoteSession] = useState<SessionPayload | null>(null);
  const [sessionReady, setSessionReady] = useState(false);
  const [uiConfig, setUiConfig] = useState<UiConfig | null>(USE_MOCKS ? DEFAULT_UI_CONFIG : null);
  const [configError, setConfigError] = useState<string | null>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  const [maxPanes, setMaxPanes] = useState(4);
  const [theme, setTheme] = useState<'light' | 'dark'>(getDefaultTheme);
  const [savedViews, setSavedViews] = useState<SavedView[]>([]);
  const [activeViewId, setActiveViewId] = useState<string | null>(null);
  const [viewFilters, setViewFilters] = useState<ViewFilters>({ logLevel: 'ALL' });
  const [logViewPrefs, setLogViewPrefs] = useState<LogViewPreferences>({
    density: 'default',
    wrap: false,
    show_timestamp: true,
    show_details: true,
    show_metrics: false
  });

  // Apply theme to document
  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
    localStorage.setItem(STORAGE_KEY_THEME, theme);
  }, [theme]);

  const handleAuth = useCallback((user: AuthUser) => {
    setSessionToken(user.accessToken ?? null);
    setAuthUser(user);
  }, []);

  useEffect(() => {
    let cancelled = false;
    if (!sessionToken) {
      setRemoteSession(null);
      setSessionReady(true);
      return;
    }

    setIsRestoring(true);
    setSessionReady(false);
    fetchSession(sessionToken)
      .then((session) => {
        if (cancelled) return;
        setRemoteSession(session);
      })
      .catch(() => {
        if (cancelled) return;
        setRemoteSession(null);
      })
      .finally(() => {
        if (cancelled) return;
        setSessionReady(true);
      });

    return () => {
      cancelled = true;
    };
  }, [sessionToken]);

  useEffect(() => {
    let cancelled = false;

    if (!sessionToken) {
      if (USE_MOCKS) {
        setUiConfig(DEFAULT_UI_CONFIG);
        setConfigError(null);
      } else {
        setUiConfig(null);
        setConfigError('Missing access token');
      }
      return;
    }

    fetchConfig(sessionToken)
      .then((cfg) => {
        if (cancelled) return;
        setUiConfig(cfg);
        setConfigError(null);
      })
      .catch((err) => {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : 'Failed to load config';
        setConfigError(message);
      });

    return () => {
      cancelled = true;
    };
  }, [sessionToken]);

  const toggleTheme = () => {
    setTheme(prev => prev === 'light' ? 'dark' : 'light');
  };

  // Handle responsive behavior
  useEffect(() => {
    const handleResize = () => {
      // Sidebar logic
      if (window.innerWidth < 1024) {
        setIsSidebarOpen(false);
      } else {
        setIsSidebarOpen(true);
      }

      // Pane limit logic
      if (window.innerWidth < 768) {
        setMaxPanes(1);
      } else if (window.innerWidth < 1280) {
        setMaxPanes(2);
      } else {
        setMaxPanes(4);
      }
    };
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Load sidebar preference from localStorage as a fallback
  useEffect(() => {
    const savedSidebar = localStorage.getItem(STORAGE_KEY_SIDEBAR);
    if (savedSidebar === 'true' || savedSidebar === 'false') {
      setIsSidebarOpen(savedSidebar === 'true');
    }
  }, []);

  // Sync state to URL and localStorage (fallback when no session token)
  useEffect(() => {
    if (isRestoring) return;

    const identifiers: ResourceIdentifier[] = activeResources.map(res => ({
      type: 'type' in res ? 'app' : 'pod',
      namespace: res.namespace,
      name: res.name
    }));

    if (!sessionToken) {
      localStorage.setItem(STORAGE_KEY_ACTIVE, JSON.stringify(identifiers));
    }
    
    // Update URL hash for linkability
    const hash = identifiers.map(i => `${i.type}:${i.namespace}:${i.name}`).join(',');
    window.location.hash = hash ? `view=${hash}` : '';
  }, [activeResources, isRestoring, sessionToken]);

  // Persist pinned (fallback when no session token)
  useEffect(() => {
    if (!sessionToken) {
      localStorage.setItem(STORAGE_KEY_PINNED, JSON.stringify(pinnedResources));
    }
  }, [pinnedResources, sessionToken]);

  useEffect(() => {
    if (!sessionToken) {
      localStorage.setItem(STORAGE_KEY_SIDEBAR, String(isSidebarOpen));
    }
  }, [isSidebarOpen, sessionToken]);

  useEffect(() => {
    if (!sessionToken) {
      localStorage.setItem(STORAGE_KEY_SAVED_VIEWS, JSON.stringify(savedViews));
      localStorage.setItem(STORAGE_KEY_VIEW_FILTERS, JSON.stringify(viewFilters));
      localStorage.setItem(STORAGE_KEY_ACTIVE_VIEW, activeViewId ?? '');
      localStorage.setItem(STORAGE_KEY_LOG_VIEW, JSON.stringify(logViewPrefs));
    }
  }, [savedViews, viewFilters, activeViewId, logViewPrefs, sessionToken]);

  useEffect(() => {
    if (isRestoring || !sessionToken) return;

    const identifiers: ResourceIdentifier[] = activeResources.map(res => ({
      type: 'type' in res ? 'app' : 'pod',
      namespace: res.namespace,
      name: res.name
    }));

    const payload: SessionPayload = {
      active_resources: identifiers,
      pinned_resources: pinnedResources,
      theme,
      sidebar_open: isSidebarOpen,
      saved_views: savedViews,
      view_filters: viewFilters,
      active_view_id: activeViewId,
      log_view: logViewPrefs
    };

    saveSession(sessionToken, payload).catch((err) => {
      console.warn('Failed to save session', err);
    });
  }, [activeResources, pinnedResources, theme, isSidebarOpen, savedViews, viewFilters, activeViewId, logViewPrefs, sessionToken, isRestoring]);

  // Restore state once session data is available
  useEffect(() => {
    if (!sessionReady) return;

    const restore = async () => {
      let idsToRestore: ResourceIdentifier[] = [];

      // Try URL first
      const hashParams = new URLSearchParams(window.location.hash.substring(1));
      const viewParam = hashParams.get('view');

      if (viewParam) {
        idsToRestore = viewParam.split(',').map(part => {
          const [type, namespace, name] = part.split(':');
          return { type: type as 'pod' | 'app', namespace, name };
        });
      } else if (sessionToken && remoteSession?.active_resources) {
        idsToRestore = remoteSession.active_resources as ResourceIdentifier[];
      } else {
        const saved = localStorage.getItem(STORAGE_KEY_ACTIVE);
        if (saved) {
          try {
            idsToRestore = JSON.parse(saved);
          } catch (e) {
            console.error("Failed to restore from storage", e);
          }
        }
      }

      if (sessionToken && remoteSession) {
        if (remoteSession.theme === 'light' || remoteSession.theme === 'dark') {
          setTheme(remoteSession.theme);
        }
        if (Array.isArray(remoteSession.pinned_resources)) {
          setPinnedResources(remoteSession.pinned_resources);
        } else {
          const savedPinned = localStorage.getItem(STORAGE_KEY_PINNED);
          if (savedPinned) {
            try {
              setPinnedResources(JSON.parse(savedPinned));
            } catch (e) {
              console.error("Failed to parse pinned resources", e);
            }
          }
        }
        if (typeof remoteSession.sidebar_open === 'boolean') {
          setIsSidebarOpen(remoteSession.sidebar_open);
        }
        if (Array.isArray(remoteSession.saved_views)) {
          setSavedViews(remoteSession.saved_views);
        }
        if (remoteSession.view_filters) {
          setViewFilters({ logLevel: 'ALL', ...remoteSession.view_filters });
        }
        if (typeof remoteSession.active_view_id === 'string' || remoteSession.active_view_id === null) {
          setActiveViewId(remoteSession.active_view_id ?? null);
        }
        if (remoteSession.log_view) {
          setLogViewPrefs(prev => ({ ...prev, ...remoteSession.log_view }));
        }
      } else {
        const savedPinned = localStorage.getItem(STORAGE_KEY_PINNED);
        if (savedPinned) {
          try {
            setPinnedResources(JSON.parse(savedPinned));
          } catch (e) {
            console.error("Failed to parse pinned resources", e);
          }
        }
        const savedViewsLocal = localStorage.getItem(STORAGE_KEY_SAVED_VIEWS);
        if (savedViewsLocal) {
          try {
            setSavedViews(JSON.parse(savedViewsLocal));
          } catch (e) {
            console.error("Failed to parse saved views", e);
          }
        }
        const savedFilters = localStorage.getItem(STORAGE_KEY_VIEW_FILTERS);
        if (savedFilters) {
          try {
            setViewFilters({ logLevel: 'ALL', ...(JSON.parse(savedFilters)) });
          } catch (e) {
            console.error("Failed to parse view filters", e);
          }
        }
        const savedActiveView = localStorage.getItem(STORAGE_KEY_ACTIVE_VIEW);
        if (savedActiveView) {
          setActiveViewId(savedActiveView || null);
        }
        const savedLogView = localStorage.getItem(STORAGE_KEY_LOG_VIEW);
        if (savedLogView) {
          try {
            setLogViewPrefs(prev => ({ ...prev, ...(JSON.parse(savedLogView)) }));
          } catch (e) {
            console.error("Failed to parse log view prefs", e);
          }
        }
      }

      if (idsToRestore.length > 0) {
        const restored: (Pod | AppResource)[] = [];
        for (const id of idsToRestore.slice(0, 4)) {
          try {
            const res = id.type === 'pod' 
              ? await getPodByName(id.namespace, id.name, sessionToken)
              : await getAppByName(id.namespace, id.name, sessionToken);
            if (res) restored.push(res);
          } catch (e) {
            console.warn(`Failed to restore ${id.name}`, e);
          }
        }
        setActiveResources(restored);
      }
      setIsRestoring(false);
    };

    restore();
  }, [sessionReady, sessionToken, remoteSession]);

  const ensureResourceDetails = useCallback(async (resource: Pod | AppResource) => {
    if (!sessionToken) return resource;
    const isApp = 'type' in resource;
    const needsDetails = resource.light || (isApp ? (resource as AppResource).podNames?.length === 0 : (resource as Pod).containers?.length === 0);
    if (!needsDetails) return resource;

    try {
      const detailed = isApp
        ? await getAppByName(resource.namespace, resource.name, sessionToken)
        : await getPodByName(resource.namespace, resource.name, sessionToken);
      return detailed || resource;
    } catch (err) {
      console.warn('Failed to load resource details', err);
      return resource;
    }
  }, [sessionToken]);

  const handleResourceSelect = useCallback((resource: Pod | AppResource) => {
    void (async () => {
      const detailed = await ensureResourceDetails(resource);
      setActiveResources(prev => {
        if (prev.some(p => p.name === detailed.name)) {
          // Move selected resource to front so it's visible in restricted grid
          return [detailed, ...prev.filter(p => p.name !== detailed.name)];
        }
        if (prev.length >= 8) { // Soft limit for total open tabs
          alert("Too many open tabs. Please close some.");
          return prev;
        }
        return [detailed, ...prev];
      });
    })();
  }, [ensureResourceDetails]);

  const closeResource = useCallback((name: string) => {
    setActiveResources(prev => prev.filter(p => p.name !== name));
  }, []);

  const togglePin = useCallback((id: ResourceIdentifier) => {
    setPinnedResources(prev => {
      const exists = prev.some(p => p.type === id.type && p.namespace === id.namespace && p.name === id.name);
      if (exists) {
        return prev.filter(p => !(p.type === id.type && p.namespace === id.namespace && p.name === id.name));
      }
      return [...prev, id];
    });
  }, []);

  const handleClearSession = useCallback(async () => {
    if (sessionToken) {
      try {
        await clearSession(sessionToken);
      } catch (e) {
        console.warn('Failed to clear session', e);
      }
    }

    localStorage.removeItem(STORAGE_KEY_ACTIVE);
    localStorage.removeItem(STORAGE_KEY_PINNED);
    localStorage.removeItem(STORAGE_KEY_SIDEBAR);
    localStorage.removeItem(STORAGE_KEY_THEME);
    localStorage.removeItem(STORAGE_KEY_SAVED_VIEWS);
    localStorage.removeItem(STORAGE_KEY_VIEW_FILTERS);
    localStorage.removeItem(STORAGE_KEY_ACTIVE_VIEW);
    localStorage.removeItem(STORAGE_KEY_LOG_VIEW);
    window.location.hash = '';

    setActiveResources([]);
    setPinnedResources([]);
    setInspectingResource(null);
    setTheme(getDefaultTheme());
    setIsSidebarOpen(true);
    setRemoteSession(null);
    setSavedViews([]);
    setViewFilters({ logLevel: 'ALL' });
    setActiveViewId(null);
    setLogViewPrefs({ density: 'default', wrap: false, show_timestamp: true, show_details: true, show_metrics: false });
  }, [sessionToken]);

  const handleSaveView = useCallback((name: string) => {
    const trimmed = name.trim();
    if (!trimmed) return;
    setSavedViews(prev => {
      const id = `${Date.now()}`;
      const next: SavedView = {
        id,
        name: trimmed,
        namespace: viewFilters.namespace,
        labelRegex: viewFilters.labelRegex,
        logLevel: viewFilters.logLevel ?? 'ALL'
      };
      return [...prev, next];
    });
  }, [viewFilters]);

  const handleApplyView = useCallback((viewId: string | null) => {
    setActiveViewId(viewId);
    const view = savedViews.find(v => v.id === viewId);
    if (!view) return;
    setViewFilters({
      namespace: view.namespace,
      labelRegex: view.labelRegex,
      logLevel: view.logLevel ?? 'ALL'
    });
  }, [savedViews]);

  const handleLogLevelChange = useCallback((level: LogLevel | 'ALL') => {
    setViewFilters(prev => ({ ...prev, logLevel: level }));
    setActiveViewId(null);
  }, []);

  const handleUpdateViewFilters = useCallback((filters: ViewFilters) => {
    setViewFilters(filters);
    setActiveViewId(null);
  }, []);

  // Determine visible resources based on current responsive limit
  const visibleResources = activeResources.slice(0, maxPanes);

  return (
    <AuthGuard onAuth={handleAuth}>
      <div className="flex h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-200 overflow-hidden font-sans transition-colors duration-200">
        <Sidebar 
          isOpen={isSidebarOpen}
          onClose={() => setIsSidebarOpen(false)}
          onPodSelect={handleResourceSelect} 
          onAppSelect={handleResourceSelect}
          activeResourceNames={activeResources.map(p => p.name)}
          pinnedIds={pinnedResources}
          onTogglePin={togglePin}
          accessToken={sessionToken}
          config={uiConfig}
          savedViews={savedViews}
          activeViewId={activeViewId}
          viewFilters={viewFilters}
          onSaveView={handleSaveView}
          onApplyView={handleApplyView}
          onUpdateViewFilters={handleUpdateViewFilters}
        />

        <main className="flex-1 flex flex-col relative overflow-hidden">
          <header className="h-14 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 flex items-center px-4 justify-between shrink-0 transition-colors duration-200">
            <div className="flex items-center gap-4 overflow-hidden h-full">
              <button 
                onClick={() => setIsSidebarOpen(!isSidebarOpen)}
                className="p-1.5 text-slate-400 hover:text-sky-500 transition-colors shrink-0"
              >
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>

              <div className="flex items-center gap-2 overflow-x-auto no-scrollbar max-w-full py-2 h-full">
                {activeResources.map((res, idx) => (
                  <div 
                    key={res.name} 
                    onClick={() => handleResourceSelect(res)}
                    className={`flex items-center border rounded-md px-2.5 py-1 gap-2 shrink-0 group cursor-pointer transition-all ${
                      idx < maxPanes 
                        ? 'bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 shadow-md dark:shadow-sky-500/5' 
                        : 'bg-white/50 dark:bg-slate-900/50 border-slate-200 dark:border-slate-800 opacity-60 hover:opacity-100'
                    }`}
                  >
                    <span className={`text-[9px] font-bold uppercase ${idx < maxPanes ? 'text-sky-500' : 'text-slate-400 dark:text-slate-500'}`}>
                      {'type' in res ? res.type[0] : 'P'}
                    </span>
                    <span className="text-[11px] font-medium text-slate-700 dark:text-slate-300 truncate max-w-[80px] md:max-w-[120px]">{res.name}</span>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button onClick={(e) => { e.stopPropagation(); setInspectingResource(res); }} className="text-slate-400 hover:text-sky-500 p-0.5">
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" strokeWidth={2}/></svg>
                      </button>
                      <button onClick={(e) => { e.stopPropagation(); closeResource(res.name); }} className="text-slate-400 hover:text-red-500 p-0.5">
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M6 18L18 6M6 6l12 12" strokeWidth={2}/></svg>
                      </button>
                    </div>
                  </div>
                ))}
                {isRestoring && (
                  <div className="flex items-center gap-2 px-3 py-1 text-[10px] text-slate-400 dark:text-slate-500 animate-pulse uppercase font-bold">
                    Restoring...
                  </div>
                )}
              </div>
            </div>
            
            <div className="flex items-center gap-2 md:gap-4 ml-4 shrink-0">
              <button 
                onClick={toggleTheme}
                className="p-2 text-slate-400 hover:text-sky-500 dark:hover:text-sky-400 rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 transition-all shadow-sm group"
                title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
              >
                {theme === 'light' ? (
                  <svg className="w-4 h-4 md:w-5 md:h-5 group-hover:rotate-12 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                  </svg>
                ) : (
                  <svg className="w-4 h-4 md:w-5 md:h-5 group-hover:rotate-45 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 9H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                  </svg>
                )}
              </button>

              <button
                onClick={handleClearSession}
                className="p-2 text-slate-400 hover:text-red-500 rounded-lg bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 transition-all shadow-sm"
                title="Clear saved session"
              >
                <svg className="w-4 h-4 md:w-5 md:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7h16M9 7V5a1 1 0 011-1h4a1 1 0 011 1v2m-1 0v12a1 1 0 01-1 1H10a1 1 0 01-1-1V7h6z" />
                </svg>
              </button>

              <div className="hidden md:flex items-center gap-2 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg px-2 py-1 text-[10px] text-slate-500 dark:text-slate-400">
                <button
                  onClick={() => setLogViewPrefs(prev => ({ ...prev, wrap: !prev.wrap }))}
                  className={`px-1.5 py-0.5 rounded uppercase font-bold ${logViewPrefs.wrap ? 'bg-sky-500/15 text-sky-500' : 'text-slate-400'}`}
                >
                  Wrap
                </button>
                <button
                  onClick={() => setLogViewPrefs(prev => ({ ...prev, show_timestamp: !prev.show_timestamp }))}
                  className={`px-1.5 py-0.5 rounded uppercase font-bold ${logViewPrefs.show_timestamp ? 'bg-sky-500/15 text-sky-500' : 'text-slate-400'}`}
                >
                  Time
                </button>
                <button
                  onClick={() => setLogViewPrefs(prev => ({ ...prev, show_details: !prev.show_details }))}
                  className={`px-1.5 py-0.5 rounded uppercase font-bold ${logViewPrefs.show_details ? 'bg-sky-500/15 text-sky-500' : 'text-slate-400'}`}
                >
                  Detail
                </button>
                <button
                  onClick={() => setLogViewPrefs(prev => ({ ...prev, show_metrics: !prev.show_metrics }))}
                  className={`px-1.5 py-0.5 rounded uppercase font-bold ${logViewPrefs.show_metrics ? 'bg-sky-500/15 text-sky-500' : 'text-slate-400'}`}
                >
                  Metrics
                </button>
                <select
                  value={logViewPrefs.density || 'default'}
                  onChange={(e) => setLogViewPrefs(prev => ({ ...prev, density: e.target.value as LogViewPreferences['density'] }))}
                  className="bg-white/70 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded px-1 py-0.5 text-[10px]"
                >
                  <option value="default">Default</option>
                  <option value="small">Small</option>
                  <option value="smaller">Smaller</option>
                  <option value="large">Large</option>
                  <option value="larger">Larger</option>
                </select>
              </div>

              <div className="flex items-center gap-3">
                <div className="hidden sm:block text-right">
                  <p className="text-xs font-bold text-slate-700 dark:text-slate-300 leading-none mb-0.5">
                    {authUser?.username || authUser?.email || 'dev_user'}
                  </p>
                  <p className="text-[9px] text-sky-600 dark:text-sky-500 font-bold uppercase tracking-tight leading-none">
                    {authUser?.groups?.[0] || 'k8s-logs-access'}
                  </p>
                </div>
                <div className="w-8 h-8 rounded-full bg-slate-100 dark:bg-slate-700 flex items-center justify-center text-sky-500 dark:text-sky-400 border border-slate-200 dark:border-slate-600 shadow-sm">
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" strokeWidth={2} /></svg>
                </div>
              </div>
            </div>
          </header>

          <div className={`flex-1 p-4 overflow-y-auto custom-scrollbar transition-colors duration-200 ${ 
            visibleResources.length === 0
              ? 'flex items-center justify-center'
              : visibleResources.length === 2
                ? 'grid grid-cols-1 grid-rows-2 auto-rows-fr gap-4'
                : visibleResources.length === 1
                  ? 'grid grid-cols-1 gap-4'
                  : 'grid grid-cols-1 xl:grid-cols-2 xl:grid-rows-2 gap-4'
          }`}>
            {configError && (
              <div className="absolute top-16 left-4 right-4 z-10 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-[10px] text-red-500">
                {configError}
              </div>
            )}
            {visibleResources.length === 0 ? (
              <div className="text-center p-6 max-w-md">
                <div className="w-20 h-20 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl flex items-center justify-center mx-auto mb-6 text-slate-300 dark:text-slate-700 shadow-sm transition-colors duration-200">
                  <svg className="w-12 h-12" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" /></svg>
                </div>
                <h2 className="text-xl md:text-2xl font-bold text-slate-800 dark:text-slate-300 mb-2 transition-colors duration-200">Welcome to KubeLens</h2>
                <p className="text-slate-500 dark:text-slate-500 mb-8 text-sm md:text-base transition-colors duration-200">Select resources in the sidebar or from the top tabs to begin debugging.</p>
              </div>
            ) : (
              visibleResources.map(res => (
                <div key={res.name} className="min-h-0 h-full">
                  <LogView 
                    resource={res} 
                    onClose={() => closeResource(res.name)}
                    isMaximized={visibleResources.length === 1}
                    accessToken={sessionToken}
                    config={uiConfig}
                    initialLogLevel={viewFilters.logLevel}
                    onLogLevelChange={handleLogLevelChange}
                    density={logViewPrefs.density}
                    globalWrap={logViewPrefs.wrap}
                    globalShowTimestamp={logViewPrefs.show_timestamp}
                    globalShowDetails={logViewPrefs.show_details}
                    globalShowMetrics={logViewPrefs.show_metrics}
                  />
                </div>
              ))
            )}
          </div>
          
          {activeResources.length > maxPanes && (
            <div className="px-4 py-1.5 bg-white/80 dark:bg-slate-900/80 border-t border-slate-200 dark:border-slate-800 text-[10px] text-slate-500 flex justify-center gap-4 transition-colors duration-200">
              <span>Showing {maxPanes} of {activeResources.length} open resources</span>
              <span className="text-sky-600 dark:text-sky-500 font-bold animate-pulse">Select a tab above to switch focus</span>
            </div>
          )}
        </main>

        {inspectingResource && (
          <PodInspector
            resource={inspectingResource}
            onClose={() => setInspectingResource(null)}
            config={uiConfig}
            accessToken={sessionToken}
            canViewSecrets={authUser?.canViewSecrets ?? false}
          />
        )}
      </div>
    </AuthGuard>
  );
};

export default App;
