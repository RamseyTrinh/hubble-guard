import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import AnomalyDetection from './pages/AnomalyDetection'
import FlowViewer from './pages/FlowViewer'
import RulesManagement from './pages/RulesManagement'

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/anomalies" element={<AnomalyDetection />} />
        <Route path="/flows" element={<FlowViewer />} />
        <Route path="/rules" element={<RulesManagement />} />
      </Routes>
    </Layout>
  )
}

export default App

