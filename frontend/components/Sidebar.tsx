
import React, { useState, useEffect, useMemo } from 'react';
import { Namespace, Pod, AppResource, ResourceIdentifier, UiConfig, SavedView, ViewFilters, LogLevel } from '../types';
import { MOCK_NAMESPACES, DEFAULT_UI_CONFIG, USE_MOCKS } from '../constants';
import { getPods, getApps, getNamespaces } from '../services/k8sService';

interface SidebarProps {
  onPodSelect: (pod: Pod) => void;
  onAppSelect: (app: AppResource) => void;
  activeResourceNames: string[];
  pinnedIds: ResourceIdentifier[];
  onTogglePin: (id: ResourceIdentifier) => void;
  isOpen: boolean;
  onClose: () => void;
  accessToken?: string | null;
  config?: UiConfig | null;
  savedViews: SavedView[];
  activeViewId: string | null;
  viewFilters: ViewFilters;
  onSaveView: (name: string) => void;
  onApplyView: (id: string | null) => void;
  onUpdateViewFilters: (filters: ViewFilters) => void;
}

type ViewMode = 'groups' | 'pods' | 'apps';

const Sidebar: React.FC<SidebarProps> = ({ 
  onPodSelect, 
  onAppSelect, 
  activeResourceNames, 
  pinnedIds,
  onTogglePin,
  isOpen,
  onClose,
  accessToken,
  config,
  savedViews,
  activeViewId,
  viewFilters,
  onSaveView,
  onApplyView,
  onUpdateViewFilters
}) => {
  const effectiveConfig = config ?? DEFAULT_UI_CONFIG;
  const appGroupsConfig = effectiveConfig.kubernetes.app_groups;
  const [viewMode, setViewMode] = useState<ViewMode>(
    appGroupsConfig.enabled ? 'groups' : 'pods'
  );
  const [namespaces, setNamespaces] = useState<Namespace[]>(USE_MOCKS ? MOCK_NAMESPACES : []);
  const [expandedNamespace, setExpandedNamespace] = useState<string | null>(null);
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null);
  
  const [pods, setPods] = useState<Record<string, Pod[]>>({});
  const [apps, setApps] = useState<Record<string, AppResource[]>>({});
  const [allApps, setAllApps] = useState<AppResource[]>([]);
  
  const [loading, setLoading] = useState<string | null>(null);
  const [isAllAppsLoading, setIsAllAppsLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isSaveModalOpen, setIsSaveModalOpen] = useState(false);
  const [saveViewName, setSaveViewName] = useState('');

  const isPinned = (type: 'pod' | 'app', namespace: string, name: string) => {
    return pinnedIds.some(id => id.type === type && id.namespace === namespace && id.name === name);
  };

  const toggleNamespace = async (ns: string) => {
    if (expandedNamespace === ns) {
      setExpandedNamespace(null);
      return;
    }
    
    setExpandedNamespace(ns);
    setLoading(ns);
    try {
      if (viewMode === 'pods' && !pods[ns]) {
        const nsPods = await getPods(ns, accessToken);
        setPods(prev => ({ ...prev, [ns]: nsPods }));
      } else if (viewMode === 'apps' && !apps[ns]) {
        const nsApps = await getApps(ns, accessToken);
        setApps(prev => ({ ...prev, [ns]: nsApps }));
      }
      setLoadError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load resources';
      setLoadError(message);
    }
    setLoading(null);
  };

  const toggleGroup = (group: string) => {
    setExpandedGroup(expandedGroup === group ? null : group);
  };

  // Fetch all apps across all namespaces for App Groups view
  useEffect(() => {
    const fetchAllApps = async () => {
      if (viewMode !== 'groups' || allApps.length > 0) return;

      setIsAllAppsLoading(true);
      try {
        const all: AppResource[] = [];
      const targetNamespaces = namespaces.length > 0 ? namespaces.map(n => n.name) : effectiveConfig.kubernetes.allowed_namespaces;
        for (const ns of targetNamespaces) {
          const nsApps = await getApps(ns, accessToken);
          all.push(...nsApps);
        }
        setAllApps(all);
        setLoadError(null);
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load apps';
        setLoadError(message);
      } finally {
        setIsAllAppsLoading(false);
      }
    };
    fetchAllApps();
  }, [viewMode, namespaces, accessToken, effectiveConfig.kubernetes.allowed_namespaces]);

  useEffect(() => {
    if (!accessToken) {
      if (USE_MOCKS) {
        setNamespaces(MOCK_NAMESPACES);
        setLoadError(null);
      } else {
        setNamespaces([]);
        setLoadError('Missing access token');
      }
      return;
    }
    getNamespaces(accessToken)
      .then((data) => {
        if (data && data.length > 0) {
          setNamespaces(data);
          setLoadError(null);
        }
      })
      .catch((err) => {
        console.warn('Failed to load namespaces', err);
        const message = err instanceof Error ? err.message : 'Failed to load namespaces';
        setLoadError(message);
      });
  }, [accessToken]);

  useEffect(() => {
    setPods({});
    setApps({});
    setAllApps([]);
  }, [accessToken]);

  useEffect(() => {
    if (appGroupsConfig.enabled && viewMode === 'pods') {
      setViewMode('groups');
    } else if (!appGroupsConfig.enabled && viewMode === 'groups') {
      setViewMode('pods');
    }
  }, [appGroupsConfig.enabled]);

  useEffect(() => {
    if (viewFilters.namespace) {
      setExpandedNamespace(viewFilters.namespace);
    }
  }, [viewFilters.namespace]);

  const visibleNamespaces = useMemo(() => {
    if (viewFilters.namespace) {
      return namespaces.filter(ns => ns.name === viewFilters.namespace);
    }
    return namespaces;
  }, [namespaces, viewFilters.namespace]);

  const labelRegex = useMemo(() => {
    if (!viewFilters.labelRegex) return null;
    try {
      return new RegExp(viewFilters.labelRegex, 'i');
    } catch {
      return null;
    }
  }, [viewFilters.labelRegex]);

  const matchesLabelFilter = useMemo(() => {
    return (labels?: Record<string, string>) => {
      if (!labelRegex) return true;
      if (!labels) return false;
      return Object.entries(labels).some(([key, value]) => {
        const sample = `${key}=${value}`;
        return labelRegex.test(sample) || labelRegex.test(key) || labelRegex.test(value);
      });
    };
  }, [labelRegex]);

  const appGroupsMap = useMemo(() => {
    if (viewMode !== 'groups') return {};

    const selector = appGroupsConfig.labels.selector;
    const nameKey = appGroupsConfig.labels.name;
    const groups: Record<string, { displayName: string; apps: AppResource[] }> = {};

    allApps.forEach((app) => {
      if (viewFilters.namespace && app.namespace !== viewFilters.namespace) {
        return;
      }
      if (!matchesLabelFilter(app.labels)) {
        return;
      }
      const groupVal = app.labels?.[selector];
      if (!groupVal) return;

      if (!groups[groupVal]) {
        groups[groupVal] = {
          displayName: app.labels?.[nameKey] || groupVal,
          apps: []
        };
      } else if (app.labels?.[nameKey] && groups[groupVal].displayName === groupVal) {
        groups[groupVal].displayName = app.labels[nameKey];
      }

      groups[groupVal].apps.push(app);
    });

    return groups;
  }, [allApps, viewMode, appGroupsConfig, viewFilters.namespace, matchesLabelFilter]);

  const sortedGroupKeys = useMemo(() => {
    return Object.keys(appGroupsMap).sort((a, b) =>
      appGroupsMap[a].displayName.localeCompare(appGroupsMap[b].displayName)
    );
  }, [appGroupsMap]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Running': case 'Ready': return 'bg-emerald-500';
      case 'Failed': case 'Error': return 'bg-red-500';
      case 'Pending': return 'bg-amber-500';
      default: return 'bg-slate-400 dark:bg-slate-500';
    }
  };

  const getAppDisplayName = (app: AppResource) => {
    const labelKey = appGroupsConfig.labels.name;
    return (
      app.labels?.[labelKey] ||
      app.labels?.['app.kubernetes.io/name'] ||
      app.labels?.['app.sgz.ai/name'] ||
      app.name
    );
  };

  const appMatchesSearch = (app: AppResource) => {
    const query = search.toLowerCase();
    if (!query) return true;
    const displayName = getAppDisplayName(app).toLowerCase();
    const namespaceName = `${app.namespace}/${app.name}`.toLowerCase();
    return displayName.includes(query) || namespaceName.includes(query) || app.name.toLowerCase().includes(query);
  };

  const getAppMetadata = (app: AppResource) => {
    const { environment, version } = appGroupsConfig.labels;
    const env = app.labels?.[environment];
    const rawVersion = app.labels?.[version] || app.image;
    const ver = rawVersion?.includes(':') ? rawVersion.split(':').pop() : rawVersion;
    return { env, ver };
  };

  return (
    <>
      {/* Mobile Overlay */}
      <div 
        className={`fixed inset-0 bg-black/40 dark:bg-black/60 backdrop-blur-sm z-[80] transition-opacity md:hidden ${isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
        onClick={onClose}
      />
      
      <aside className={`fixed md:relative z-[90] w-72 bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 flex flex-col h-full shrink-0 transition-all duration-300 ease-in-out ${isOpen ? 'translate-x-0' : '-translate-x-full md:hidden'}`}>
        <div className="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between transition-colors duration-200">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-sky-500 rounded flex items-center justify-center shadow-lg shadow-sky-500/20">
              <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
            </div>
            <h1 className="text-xl font-bold text-slate-800 dark:text-white tracking-tight">KubeLens</h1>
          </div>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-white md:hidden">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M6 18L18 6M6 6l12 12" strokeWidth={2} /></svg>
          </button>
        </div>

        <div className="p-4 border-b border-slate-200 dark:border-slate-800 transition-colors duration-200">
          <div className="flex p-1 bg-slate-50 dark:bg-slate-950 rounded-lg border border-slate-200 dark:border-slate-800 mb-4 transition-colors duration-200 overflow-x-auto no-scrollbar">
            {appGroupsConfig.enabled && (
              <button 
                onClick={() => setViewMode('groups')}
                className={`flex-1 min-w-[80px] py-1.5 text-[9px] font-bold uppercase tracking-wider rounded-md transition-all ${viewMode === 'groups' ? 'bg-white dark:bg-slate-800 text-sky-600 dark:text-sky-400 shadow-sm border border-slate-200 dark:border-slate-700' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}`}
              >
                Groups
              </button>
            )}
            <button 
              onClick={() => setViewMode('apps')}
              className={`flex-1 min-w-[80px] py-1.5 text-[9px] font-bold uppercase tracking-wider rounded-md transition-all ${viewMode === 'apps' ? 'bg-white dark:bg-slate-800 text-sky-600 dark:text-sky-400 shadow-sm border border-slate-200 dark:border-slate-700' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}`}
            >
              Apps
            </button>
            <button 
              onClick={() => setViewMode('pods')}
              className={`flex-1 min-w-[80px] py-1.5 text-[9px] font-bold uppercase tracking-wider rounded-md transition-all ${viewMode === 'pods' ? 'bg-white dark:bg-slate-800 text-sky-600 dark:text-sky-400 shadow-sm border border-slate-200 dark:border-slate-700' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}`}
            >
              Pods
            </button>
          </div>

          <div className="relative">
            <input
              type="text"
              placeholder={`Filter ${viewMode}...`}
              className="w-full bg-slate-50 dark:bg-slate-800 text-sm pl-9 pr-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500/50 text-slate-700 dark:text-slate-200 transition-all"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <svg className="w-4 h-4 text-slate-400 dark:text-slate-500 absolute left-3 top-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>

          <div className="mt-3 space-y-2">
            <div className="flex items-center gap-2">
              <select
                value={activeViewId || ''}
                onChange={(e) => onApplyView(e.target.value || null)}
                className="flex-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-300"
              >
                <option value="">Saved views…</option>
                {savedViews.map(view => (
                  <option key={view.id} value={view.id}>{view.name}</option>
                ))}
              </select>
              <button
                onClick={() => {
                  setSaveViewName('');
                  setIsSaveModalOpen(true);
                }}
                className="px-2 py-1 text-[10px] font-bold uppercase rounded bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-500 hover:text-sky-500"
              >
                Save
              </button>
            </div>

            <div className="flex items-center gap-2">
              <select
                value={viewFilters.namespace || ''}
                onChange={(e) => onUpdateViewFilters({ ...viewFilters, namespace: e.target.value || undefined })}
                className="flex-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-300"
              >
                <option value="">All namespaces</option>
                {namespaces.map(ns => (
                  <option key={ns.name} value={ns.name}>{ns.name}</option>
                ))}
              </select>
              <input
                type="text"
                placeholder="Label/regex"
                value={viewFilters.labelRegex || ''}
                onChange={(e) => onUpdateViewFilters({ ...viewFilters, labelRegex: e.target.value })}
                className="flex-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-300"
              />
            </div>

            <div className="flex items-center gap-2">
              <select
                value={viewFilters.logLevel || 'ALL'}
                onChange={(e) => onUpdateViewFilters({ ...viewFilters, logLevel: e.target.value as LogLevel | 'ALL' })}
                className="flex-1 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded px-2 py-1 text-[10px] text-slate-600 dark:text-slate-300"
              >
                <option value="ALL">All levels</option>
                <option value="INFO">Info</option>
                <option value="WARNING">Warn</option>
                <option value="ERROR">Error</option>
                <option value="DEBUG">Debug</option>
              </select>
            </div>
          </div>
        </div>

        {loadError && (
          <div className="mx-4 mb-2 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-[10px] text-red-500">
            {loadError}
          </div>
        )}

        <div className="flex-1 overflow-y-auto py-2 custom-scrollbar transition-colors duration-200">
          {/* Pinned Section */}
          {pinnedIds.length > 0 && (
            <div className="mb-4">
              <div className="px-4 py-2 flex items-center gap-2 text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">
                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M10 2a1 1 0 011 1v1.323l3.954 1.582 1.599-.8a1 1 0 01.894 1.789l-1.322.661V12a1 1 0 01-1 1H7.018a5.501 5.501 0 01-5.5-5.5c0-3.037 2.463-5.5 5.5-5.5V3a1 1 0 011-1h2zM5.501 8A3.5 3.5 0 002 11.5a3.5 3.5 0 107 0V8h-3.5z" /></svg>
                Pinned Resources
              </div>
              <div className="space-y-0.5 px-2">
                {pinnedIds.map((id) => (
                  <div key={`${id.type}-${id.namespace}-${id.name}`} className="group relative">
                    <button
                      onClick={() => {
                        if (id.type === 'pod') {
                          onPodSelect({ name: id.name, namespace: id.namespace, status: 'Running' } as Pod);
                        } else {
                          onAppSelect({ name: id.name, namespace: id.namespace, type: 'Deployment', replicas: 1, readyReplicas: 1 } as AppResource);
                        }
                        if (window.innerWidth < 768) onClose();
                      }}
                      className={`w-full flex items-center justify-between px-3 py-2 rounded-md text-[11px] transition-all border ${
                        activeResourceNames.includes(id.name) 
                          ? 'bg-sky-50 dark:bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-200 dark:border-sky-500/30' 
                          : 'text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/80 hover:text-slate-900 dark:hover:text-slate-200 border-transparent'
                      }`}
                    >
                      <div className="flex flex-col items-start leading-tight overflow-hidden text-left">
                        <span className="truncate font-medium w-full">{id.name}</span>
                        <span className="text-[8px] text-slate-400 dark:text-slate-500 uppercase">{id.namespace} / {id.type}</span>
                      </div>
                    </button>
                    <button 
                      onClick={(e) => { e.stopPropagation(); onTogglePin(id); }}
                      className="absolute right-2 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 p-1 text-slate-400 hover:text-sky-500"
                    >
                      <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M5 4a2 2 0 012-2h6a2 2 0 012 2v.06a2 2 0 011.87 1.96v.3l-1.2 4.8a2 2 0 01-1.93 1.54H6.26a2 2 0 01-1.93-1.54L3.13 6.32v-.3a2 2 0 011.87-1.96V4zM5 13a1 1 0 011-1h8a1 1 0 110 2H6a1 1 0 01-1-1z" /></svg>
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {viewMode === 'groups' ? (
            isAllAppsLoading ? (
              <div className="px-8 py-4 flex items-center gap-3 text-xs text-slate-400 dark:text-slate-500 italic">
                <div className="w-3 h-3 rounded-full bg-sky-500 animate-pulse"></div>
                Analyzing App Groups...
              </div>
            ) : sortedGroupKeys.length === 0 ? (
              <div className="px-8 py-4 text-xs text-slate-400 dark:text-slate-500 italic">
                No app groups available
              </div>
            ) : (
              sortedGroupKeys.map((group) => {
                const groupDisplayName = appGroupsMap[group].displayName;
                const query = search.toLowerCase();
                const groupMatches = !query || groupDisplayName.toLowerCase().includes(query);
                const filteredApps = groupMatches
                  ? appGroupsMap[group].apps
                  : appGroupsMap[group].apps.filter(appMatchesSearch);

                if (filteredApps.length === 0) return null;

                return (
                <div key={group} className="mb-1">
                  <button
                    onClick={() => toggleGroup(group)}
                    className={`w-full flex items-center gap-2 px-4 py-2 text-sm font-semibold transition-all group ${
                      expandedGroup === group ? 'text-sky-600 dark:text-sky-400 bg-slate-50 dark:bg-slate-800/50' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800/30'
                    }`}
                  >
                    <svg 
                      className={`w-4 h-4 transition-transform duration-200 ${expandedGroup === group ? 'rotate-90 text-sky-500' : 'text-slate-300 dark:text-slate-600 group-hover:text-slate-500 dark:hover:text-slate-400'}`} 
                      fill="none" viewBox="0 0 24 24" stroke="currentColor"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M9 5l7 7-7 7" />
                    </svg>
                    <span className="truncate">{groupDisplayName}</span>
                  </button>

                  {expandedGroup === group && (
                    <div className="pl-8 pr-3 mt-1 space-y-0.5">
                      {filteredApps.map((app) => {
                        const { env, ver } = getAppMetadata(app);
                        const childLabel = `${app.namespace}/${app.name}`;
                        return (
                          <div key={`${app.namespace}-${app.name}`} className="group relative">
                            <button
                              onClick={() => {
                                onAppSelect(app);
                                if (window.innerWidth < 768) onClose();
                              }}
                              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[11px] transition-all border ${
                                activeResourceNames.includes(app.name) 
                                  ? 'bg-sky-50 dark:bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-200 dark:border-sky-500/30' 
                                  : 'text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/80 hover:text-slate-900 dark:hover:text-slate-200 border-transparent'
                              }`}
                            >
                              <span className={`w-2 h-2 rounded shrink-0 ${app.readyReplicas === app.replicas ? 'bg-emerald-500' : 'bg-amber-500'}`}></span>
                              <div className="flex flex-col items-start leading-tight overflow-hidden text-left">
                                <span className="truncate font-medium w-full">{childLabel}</span>
                                <div className="flex gap-1 mt-0.5 flex-wrap">
                                  <span className="text-[7px] font-bold px-1 bg-slate-500/10 text-slate-500 border border-slate-500/20 rounded uppercase leading-none py-0.5">
                                    {app.type}
                                  </span>
                                  {env && (
                                    <span className="text-[7px] font-bold px-1 bg-sky-500/10 text-sky-500 border border-sky-500/20 rounded uppercase leading-none py-0.5">
                                      {env}
                                    </span>
                                  )}
                                  {ver && (
                                    <span className="text-[7px] font-bold px-1 bg-slate-500/10 text-slate-500 border border-slate-500/20 rounded leading-none py-0.5">
                                      {ver}
                                    </span>
                                  )}
                                </div>
                              </div>
                            </button>
                            <button 
                              onClick={(e) => { e.stopPropagation(); onTogglePin({ type: 'app', namespace: app.namespace, name: app.name }); }}
                              className={`absolute right-1 top-1/2 -translate-y-1/2 p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 transition-all ${isPinned('app', app.namespace, app.name) ? 'opacity-100 text-sky-500 dark:text-sky-400' : 'opacity-0 group-hover:opacity-100 text-slate-400 dark:text-slate-500 hover:text-sky-500 dark:hover:text-sky-400'}`}
                            >
                              <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M5 4a2 2 0 012-2h6a2 2 0 012 2v.06a2 2 0 011.87 1.96v.3l-1.2 4.8a2 2 0 01-1.93 1.54H6.26a2 2 0 01-1.93-1.54L3.13 6.32v-.3a2 2 0 011.87-1.96V4zM5 13a1 1 0 011-1h8a1 1 0 110 2H6a1 1 0 01-1-1z" /></svg>
                            </button>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
                );
              })
            )
          ) : (
            visibleNamespaces.map((ns) => (
              <div key={ns.name} className="mb-1">
                <button
                  onClick={() => toggleNamespace(ns.name)}
                  className={`w-full flex items-center gap-2 px-4 py-2 text-sm font-semibold transition-all group ${
                    expandedNamespace === ns.name ? 'text-sky-600 dark:text-sky-400 bg-slate-50 dark:bg-slate-800/50' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800/30'
                  }`}
                >
                  <svg 
                    className={`w-4 h-4 transition-transform duration-200 ${expandedNamespace === ns.name ? 'rotate-90 text-sky-500' : 'text-slate-300 dark:text-slate-600 group-hover:text-slate-500 dark:group-hover:text-slate-400'}`} 
                    fill="none" viewBox="0 0 24 24" stroke="currentColor"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M9 5l7 7-7 7" />
                  </svg>
                  <span className="truncate">{ns.name}</span>
                </button>

                {expandedNamespace === ns.name && (
                  <div className="pl-8 pr-3 mt-1 space-y-0.5">
                    {loading === ns.name ? (
                      <div className="py-2 flex items-center gap-2 text-[11px] text-slate-400 dark:text-slate-500 italic">
                        <div className="w-2 h-2 rounded-full bg-sky-500 animate-ping"></div>
                        Loading...
                      </div>
                    ) : (
                      viewMode === 'pods' ? (
                        pods[ns.name]
                          ?.filter(p => p.name.toLowerCase().includes(search.toLowerCase()))
                          .filter(p => matchesLabelFilter(p.labels))
                          .map((pod) => (
                          <div key={pod.name} className="group relative">
                            <button
                              onClick={() => {
                                onPodSelect(pod);
                                if (window.innerWidth < 768) onClose();
                              }}
                              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[11px] transition-all border ${
                                activeResourceNames.includes(pod.name) 
                                  ? 'bg-sky-50 dark:bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-200 dark:border-sky-500/30' 
                                  : 'text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/80 hover:text-slate-900 dark:hover:text-slate-200 border-transparent'
                              }`}
                            >
                              <span className={`w-2 h-2 rounded-full ${getStatusColor(pod.status)} shrink-0`}></span>
                              <span className="truncate font-medium">{pod.name}</span>
                            </button>
                            <button 
                              onClick={(e) => { e.stopPropagation(); onTogglePin({ type: 'pod', namespace: pod.namespace, name: pod.name }); }}
                              className={`absolute right-1 top-1/2 -translate-y-1/2 p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 transition-all ${isPinned('pod', pod.namespace, pod.name) ? 'opacity-100 text-sky-500 dark:text-sky-400' : 'opacity-0 group-hover:opacity-100 text-slate-400 dark:text-slate-500 hover:text-sky-500 dark:hover:text-sky-400'}`}
                            >
                              <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M5 4a2 2 0 012-2h6a2 2 0 012 2v.06a2 2 0 011.87 1.96v.3l-1.2 4.8a2 2 0 01-1.93 1.54H6.26a2 2 0 01-1.93-1.54L3.13 6.32v-.3a2 2 0 011.87-1.96V4zM5 13a1 1 0 011-1h8a1 1 0 110 2H6a1 1 0 01-1-1z" /></svg>
                            </button>
                          </div>
                        ))
                      ) : (
                        apps[ns.name]
                          ?.filter(appMatchesSearch)
                          .filter(app => matchesLabelFilter(app.labels))
                          .map((app) => {
                            const isMetaOnly = app.metadataOnly;
                            const statusColor = isMetaOnly
                              ? 'bg-slate-400 dark:bg-slate-500'
                              : (app.readyReplicas === app.replicas && app.replicas > 0 ? 'bg-emerald-500' : 'bg-amber-500');
                            return (
                          <div key={app.name} className="group relative">
                            <button
                              onClick={() => {
                                onAppSelect(app);
                                if (window.innerWidth < 768) onClose();
                              }}
                              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[11px] transition-all border ${
                                activeResourceNames.includes(app.name) 
                                  ? 'bg-sky-50 dark:bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-200 dark:border-sky-500/30' 
                                  : 'text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/80 hover:text-slate-900 dark:hover:text-slate-200 border-transparent'
                              }`}
                            >
                              <span className={`w-2 h-2 rounded shrink-0 ${statusColor}`}></span>
                              <div className="flex flex-col items-start leading-tight overflow-hidden text-left">
                                <span className="truncate font-medium w-full">{getAppDisplayName(app)}</span>
                                <span className="text-[8px] text-slate-400 dark:text-slate-500 uppercase">
                                  {app.type} ({isMetaOnly ? '—' : `${app.readyReplicas}/${app.replicas}`})
                                </span>
                              </div>
                            </button>
                            <button 
                              onClick={(e) => { e.stopPropagation(); onTogglePin({ type: 'app', namespace: app.namespace, name: app.name }); }}
                              className={`absolute right-1 top-1/2 -translate-y-1/2 p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 transition-all ${isPinned('app', app.namespace, app.name) ? 'opacity-100 text-sky-500 dark:text-sky-400' : 'opacity-0 group-hover:opacity-100 text-slate-400 dark:text-slate-500 hover:text-sky-500 dark:hover:text-sky-400'}`}
                            >
                              <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M5 4a2 2 0 012-2h6a2 2 0 012 2v.06a2 2 0 011.87 1.96v.3l-1.2 4.8a2 2 0 01-1.93 1.54H6.26a2 2 0 01-1.93-1.54L3.13 6.32v-.3a2 2 0 011.87-1.96V4zM5 13a1 1 0 011-1h8a1 1 0 110 2H6a1 1 0 01-1-1z" /></svg>
                            </button>
                          </div>
                        )})
                      )
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
        
        <div className="p-4 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/50 transition-colors duration-200">
          <div className="flex items-center justify-between text-[10px] mb-2">
            <span className="text-slate-400 dark:text-slate-500 uppercase tracking-widest font-bold">Session</span>
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"></span>
          </div>
          <div className="text-[10px] text-slate-500 dark:text-slate-400 truncate">
            {effectiveConfig.kubernetes.cluster_name || 'cluster'}
          </div>
        </div>
      </aside>

      {isSaveModalOpen && (
        <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50 px-4">
          <div className="w-full max-w-sm rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 shadow-2xl">
            <div className="px-5 py-4 border-b border-slate-200 dark:border-slate-800">
              <h3 className="text-sm font-bold text-slate-800 dark:text-slate-200">Save current view</h3>
              <p className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                Store the current namespace, label, and log level filters.
              </p>
            </div>
            <div className="px-5 py-4">
              <label className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wide">View name</label>
              <input
                autoFocus
                value={saveViewName}
                onChange={(e) => setSaveViewName(e.target.value)}
                placeholder="e.g. Flipco errors"
                className="mt-2 w-full rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/70 px-3 py-2 text-sm text-slate-700 dark:text-slate-200 focus:border-sky-500 focus:ring-1 focus:ring-sky-500/40"
              />
            </div>
            <div className="flex items-center justify-end gap-2 px-5 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60">
              <button
                onClick={() => {
                  setIsSaveModalOpen(false);
                  setSaveViewName('');
                }}
                className="px-3 py-1.5 text-xs font-bold uppercase text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  const trimmed = saveViewName.trim();
                  if (!trimmed) return;
                  onSaveView(trimmed);
                  setIsSaveModalOpen(false);
                  setSaveViewName('');
                }}
                className={`px-4 py-1.5 text-xs font-bold uppercase rounded-md ${
                  saveViewName.trim()
                    ? 'bg-sky-500 text-white hover:bg-sky-400'
                    : 'bg-slate-200 text-slate-400 dark:bg-slate-800 dark:text-slate-600 cursor-not-allowed'
                }`}
                disabled={!saveViewName.trim()}
              >
                Save view
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default Sidebar;
