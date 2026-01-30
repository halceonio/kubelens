
import React, { useState, useMemo } from 'react';
import { Pod, AppResource } from '../types';
import { MOCK_CONFIG } from '../constants';

interface PodInspectorProps {
  resource: Pod | AppResource;
  onClose: () => void;
}

type TabType = 'ENV' | 'LABELS' | 'VOLUMES' | 'RESOURCES' | 'METRICS' | 'PODS';

const PodInspector: React.FC<PodInspectorProps> = ({ resource, onClose }) => {
  const [activeTab, setActiveTab] = useState<TabType>('METRICS');
  const isApp = resource && 'type' in resource;

  if (!resource) return null;

  // Extract prefixed metadata for specialized display
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

  const parseResource = (val: string) => {
    if (!val) return 0;
    const num = parseFloat(val);
    const lowVal = val.toLowerCase();
    if (lowVal.endsWith('m')) return num / 1000;
    if (lowVal.endsWith('mi')) return num;
    if (lowVal.endsWith('gi')) return num * 1024;
    return num;
  };

  const calculatePercent = (usage: string, limit: string) => {
    const u = parseResource(usage);
    const l = parseResource(limit);
    if (l <= 0) return 0;
    return Math.min(100, (u / l) * 100);
  };

  const renderResources = () => {
    const res = resource.resources || { cpuUsage: '0', cpuLimit: '1', memUsage: '0', memLimit: '1', cpuRequest: '0', memRequest: '0' };
    const cpuPerc = calculatePercent(res.cpuUsage, res.cpuLimit);
    const memPerc = calculatePercent(res.memUsage, res.memLimit);

    const ProgressBar = ({ label, current, request, limit, percent, color }: any) => (
      <div className="mb-8 last:mb-0">
        <div className="flex justify-between items-end mb-2">
          <div>
            <h4 className="text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest mb-1">
              {isApp ? 'Aggregated ' : ''}{label} Allocation
            </h4>
            <span className="text-lg font-bold text-slate-800 dark:text-white mono transition-colors duration-200">{current || 'N/A'}</span>
            <span className="text-xs text-slate-500 ml-2">used of {limit || 'unlimited'} limit</span>
          </div>
          <div className="text-right">
            <span className={`text-sm font-bold ${percent > 85 ? 'text-red-500 dark:text-red-400' : percent > 60 ? 'text-amber-500 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'}`}>{percent.toFixed(1)}%</span>
          </div>
        </div>
        <div className="h-3 bg-slate-100 dark:bg-slate-900 rounded-full border border-slate-200 dark:border-slate-700 overflow-hidden relative transition-colors duration-200">
          <div 
            className={`h-full transition-all duration-1000 rounded-full ${color}`} 
            style={{ width: `${percent}%` }}
          />
          {limit && limit !== '0' && (
            <div 
              className="absolute top-0 bottom-0 border-r-2 border-slate-300 dark:border-white/20 z-10" 
              style={{ left: `${(parseResource(request) / parseResource(limit)) * 100}%` }}
              title={`Request: ${request}`}
            />
          )}
        </div>
        <div className="flex justify-between mt-1 text-[9px] text-slate-400 dark:text-slate-500 uppercase font-bold mono">
          <span>0</span>
          <span>Req: {request || 'None'}</span>
          <span>Lim: {limit || 'None'}</span>
        </div>
      </div>
    );

    return (
      <div className="space-y-4">
        {Object.keys(enterpriseMetadata).length > 0 && (
          <div className="p-4 bg-sky-50 dark:bg-sky-500/5 rounded-xl border border-sky-200 dark:border-sky-500/20 transition-colors duration-200">
            <h4 className="text-[10px] font-bold text-sky-600 dark:text-sky-400 uppercase tracking-widest mb-3 flex items-center gap-2">
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" strokeWidth={2}/></svg>
              Enterprise Context
            </h4>
            <div className="grid grid-cols-2 gap-3">
              {Object.entries(enterpriseMetadata).map(([key, value]) => (
                <div key={key} className="flex flex-col">
                  <span className="text-[9px] font-bold text-slate-400 dark:text-slate-500 uppercase">{key}</span>
                  <span className="text-xs text-sky-700 dark:text-sky-200 font-medium truncate" title={value}>{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        
        <div className="mt-4 p-4 bg-white dark:bg-slate-900/50 rounded-xl border border-slate-200 dark:border-slate-700/50 shadow-sm transition-colors duration-200">
          <ProgressBar label="CPU" current={res.cpuUsage} request={res.cpuRequest} limit={res.cpuLimit} percent={cpuPerc} color="bg-sky-500 shadow-sky-500/20" />
          <div className="h-px bg-slate-100 dark:bg-slate-700/30 my-6" />
          <ProgressBar label="Memory" current={res.memUsage} request={res.memRequest} limit={res.memLimit} percent={memPerc} color="bg-fuchsia-500 shadow-fuchsia-500/20" />
        </div>
      </div>
    );
  };

  const renderContent = () => {
    switch (activeTab) {
      case 'ENV': return renderKeyValue(resource.env || {});
      case 'LABELS': return renderKeyValue({ ...(resource.labels || {}), ...(resource.annotations || {}) });
      case 'VOLUMES':
        return (
          <div className="mt-4 space-y-4">
             {(resource.volumes || []).length > 0 ? resource.volumes?.map((v, i) => (
               <div key={i} className="bg-slate-50 dark:bg-slate-900 p-3 rounded-lg border border-slate-200 dark:border-slate-700 flex justify-between items-center text-xs transition-colors duration-200">
                  <div className="flex flex-col">
                    <span className="text-sky-600 dark:text-sky-400 font-bold mono">{v.name}</span>
                    <span className="text-[10px] text-slate-400 dark:text-slate-500">{v.readOnly ? 'ReadOnly' : 'ReadWrite'}</span>
                  </div>
                  <span className="text-slate-600 dark:text-slate-400">{v.mountPath}</span>
               </div>
             )) : <div className="text-center py-8 text-slate-400 italic">No volumes mounted</div>}
          </div>
        );
      case 'RESOURCES':
        return (
          <div className="mt-4 grid grid-cols-2 gap-4">
            <div className="bg-slate-50 dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-slate-700 transition-colors duration-200">
               <h4 className="text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase mb-3">Secrets</h4>
               {(resource.secrets || []).length > 0 ? resource.secrets?.map(s => <div key={s} className="text-xs text-slate-600 dark:text-slate-300 mono py-1">🔒 {s}</div>) : <div className="text-[10px] text-slate-400 dark:text-slate-600 italic">None</div>}
            </div>
            <div className="bg-slate-50 dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-slate-700 transition-colors duration-200">
               <h4 className="text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase mb-3">ConfigMaps</h4>
               {(resource.configMaps || []).length > 0 ? resource.configMaps?.map(c => <div key={c} className="text-xs text-slate-600 dark:text-slate-300 mono py-1">📄 {c}</div>) : <div className="text-[10px] text-slate-400 dark:text-slate-600 italic">None</div>}
            </div>
          </div>
        );
      case 'METRICS': return renderResources();
      case 'PODS':
        return isApp ? (
          <div className="mt-4 space-y-2">
            {((resource as AppResource).podNames || []).map(p => (
              <div key={p} className="bg-slate-50 dark:bg-slate-900 p-3 rounded-lg border border-slate-200 dark:border-slate-700 flex justify-between items-center text-[11px] mono transition-colors duration-200">
                <span className="text-slate-600 dark:text-slate-300">{p}</span>
                <span className="text-emerald-600 dark:text-emerald-500 uppercase font-bold text-[9px]">Running</span>
              </div>
            ))}
          </div>
        ) : null;
      default: return null;
    }
  };

  const renderKeyValue = (data: Record<string, string> | undefined | null) => {
    const entries = data ? Object.entries(data) : [];
    return (
      <div className="mt-4 bg-slate-50 dark:bg-slate-900 rounded-xl p-4 mono text-[11px] border border-slate-200 dark:border-slate-700 max-h-[400px] overflow-y-auto custom-scrollbar transition-colors duration-200">
        {entries.length > 0 ? entries.map(([k, v]) => (
          <div key={k} className="flex border-b border-slate-200 dark:border-slate-800 last:border-0 py-2">
            <span className="text-sky-600 dark:text-sky-400 font-bold shrink-0 w-32 truncate" title={k}>{k}:</span>
            <span className="text-slate-700 dark:text-slate-300 break-all">{v}</span>
          </div>
        )) : <div className="text-slate-400 italic">No entries found</div>}
      </div>
    );
  };

  const tabs = [
    { id: 'METRICS', label: 'Overview' },
    { id: 'ENV', label: 'Env' },
    { id: 'LABELS', label: 'Labels' },
    { id: 'VOLUMES', label: 'Volumes' },
    { id: 'RESOURCES', label: 'Configs' }
  ];
  if (isApp) tabs.push({ id: 'PODS', label: 'Replicas' });

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 dark:bg-black/80 backdrop-blur-sm p-4 transition-all">
      <div className="bg-white dark:bg-slate-800 w-full max-w-2xl rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-700 flex flex-col max-h-[90vh] overflow-hidden transition-colors duration-200">
        <div className="p-5 border-b border-slate-200 dark:border-slate-700 flex justify-between items-center bg-slate-50 dark:bg-slate-900/50 transition-colors duration-200">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-white dark:bg-slate-800 rounded-lg flex items-center justify-center border border-slate-200 dark:border-slate-700 text-sky-500 transition-colors duration-200">
              <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" strokeWidth={2}/>
              </svg>
            </div>
            <div>
              <h2 className="text-lg font-bold text-slate-800 dark:text-white flex items-center gap-2 transition-colors duration-200">
                {resource.name}
                {isApp && <span className="text-[10px] bg-sky-100 dark:bg-sky-500/20 text-sky-600 dark:text-sky-400 px-1.5 py-0.5 rounded border border-sky-200 dark:border-sky-500/30">{resource.type?.toUpperCase()}</span>}
              </h2>
              <span className="text-[10px] text-slate-400 dark:text-slate-500 uppercase font-bold tracking-widest">{resource.namespace}</span>
            </div>
          </div>
          <button onClick={onClose} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-white rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path d="M6 18L18 6M6 6l12 12" strokeWidth={2.5}/></svg>
          </button>
        </div>

        <div className="flex gap-1 px-5 border-b border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 overflow-x-auto no-scrollbar transition-colors duration-200">
          {tabs.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as TabType)}
              className={`px-4 py-3 text-[10px] font-bold uppercase tracking-widest transition-all border-b-2 whitespace-nowrap ${activeTab === tab.id ? 'border-sky-500 text-sky-600 dark:text-sky-400' : 'border-transparent text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:hover:text-slate-300'}`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="p-5 overflow-y-auto custom-scrollbar flex-1 bg-white dark:bg-slate-800 transition-colors duration-200">
          {renderContent()}
        </div>

        <div className="p-5 border-t border-slate-200 dark:border-slate-700 text-right bg-slate-50 dark:bg-slate-900/30 transition-colors duration-200">
          <button onClick={onClose} className="px-6 py-2 bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 rounded-lg text-xs font-bold uppercase tracking-widest text-slate-700 dark:text-white transition-all">Close</button>
        </div>
      </div>
    </div>
  );
};

export default PodInspector;
