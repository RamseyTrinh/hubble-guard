import { create } from 'zustand'

const useStore = create((set) => ({
  // Flows state
  flows: [],
  flowsLoading: false,
  flowsError: null,
  
  // Alerts state
  alerts: [],
  alertsLoading: false,
  alertsError: null,
  
  // Rules state
  rules: [],
  rulesLoading: false,
  rulesError: null,
  
  // Stats state
  stats: null,
  statsLoading: false,
  
  // Actions
  setFlows: (flows) => set({ flows }),
  setFlowsLoading: (loading) => set({ flowsLoading: loading }),
  setFlowsError: (error) => set({ flowsError: error }),
  
  setAlerts: (alerts) => set({ alerts }),
  addAlert: (alert) => set((state) => ({ alerts: [alert, ...state.alerts] })),
  setAlertsLoading: (loading) => set({ alertsLoading: loading }),
  setAlertsError: (error) => set({ alertsError: error }),
  
  setRules: (rules) => set({ rules }),
  updateRule: (id, data) => set((state) => ({
    rules: state.rules.map((rule) => 
      rule.id === id ? { ...rule, ...data } : rule
    ),
  })),
  setRulesLoading: (loading) => set({ rulesLoading: loading }),
  setRulesError: (error) => set({ rulesError: error }),
  
  setStats: (stats) => set({ stats }),
  setStatsLoading: (loading) => set({ statsLoading: loading }),
}))

export default useStore

