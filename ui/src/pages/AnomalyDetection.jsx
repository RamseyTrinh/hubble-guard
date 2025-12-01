import { useEffect, useState } from 'react'
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  TextField,
  MenuItem,
  Select,
  FormControl,
  InputLabel,
  CircularProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
} from '@mui/material'
import { Visibility, Refresh } from '@mui/icons-material'
import { format } from 'date-fns'
import { alertsAPI, createWebSocket } from '../services/api'
import useStore from '../store/useStore'

const severityColors = {
  CRITICAL: 'error',
  HIGH: 'warning',
  MEDIUM: 'info',
  LOW: 'default',
}

export default function AnomalyDetection() {
  const { alerts, setAlerts, addAlert, alertsLoading, setAlertsLoading, alertsError, setAlertsError } = useStore()
  const [severityFilter, setSeverityFilter] = useState('ALL')
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedAlert, setSelectedAlert] = useState(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  useEffect(() => {
    loadAlerts()
    setupWebSocket()
    const interval = setInterval(loadAlerts, 10000)
    return () => clearInterval(interval)
  }, [])

  const loadAlerts = async () => {
    try {
      setAlertsLoading(true)
      const params = {}
      if (severityFilter !== 'ALL') {
        params.severity = severityFilter
      }
      if (searchTerm) {
        params.search = searchTerm
      }
      const response = await alertsAPI.getAll(params)
      setAlerts(response.data?.items || response.data || [])
      setAlertsError(null)
    } catch (err) {
      setAlertsError(err.message)
      console.error('Failed to load alerts:', err)
    } finally {
      setAlertsLoading(false)
    }
  }

  const setupWebSocket = () => {
    try {
      const ws = createWebSocket('/stream/alerts')
      ws.onmessage = (event) => {
        const alert = JSON.parse(event.data)
        addAlert(alert)
      }
      ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }
      return () => ws.close()
    } catch (err) {
      console.error('Failed to setup WebSocket:', err)
    }
  }

  const filteredAlerts = alerts.filter((alert) => {
    if (severityFilter !== 'ALL' && alert.severity !== severityFilter) {
      return false
    }
    if (searchTerm && !alert.message?.toLowerCase().includes(searchTerm.toLowerCase())) {
      return false
    }
    return true
  })

  const handleViewDetails = (alert) => {
    setSelectedAlert(alert)
    setDialogOpen(true)
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4">Anomaly Detection</Typography>
        <IconButton onClick={loadAlerts} disabled={alertsLoading}>
          <Refresh />
        </IconButton>
      </Box>

      {alertsError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {alertsError}
        </Alert>
      )}

      <Paper sx={{ p: 2, mb: 2 }}>
        <Box display="flex" gap={2} alignItems="center">
          <TextField
            label="Search"
            variant="outlined"
            size="small"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            sx={{ flexGrow: 1 }}
          />
          <FormControl size="small" sx={{ minWidth: 150 }}>
            <InputLabel>Severity</InputLabel>
            <Select
              value={severityFilter}
              label="Severity"
              onChange={(e) => setSeverityFilter(e.target.value)}
            >
              <MenuItem value="ALL">All</MenuItem>
              <MenuItem value="CRITICAL">Critical</MenuItem>
              <MenuItem value="HIGH">High</MenuItem>
              <MenuItem value="MEDIUM">Medium</MenuItem>
              <MenuItem value="LOW">Low</MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Paper>

      {alertsLoading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Timestamp</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Message</TableCell>
                <TableCell>Namespace</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filteredAlerts.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <Typography variant="body2" color="textSecondary">
                      No alerts found
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredAlerts.map((alert, index) => (
                  <TableRow key={alert.id || index} hover>
                    <TableCell>
                      {alert.timestamp
                        ? format(new Date(alert.timestamp), 'yyyy-MM-dd HH:mm:ss')
                        : '-'}
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={alert.severity || 'UNKNOWN'}
                        color={severityColors[alert.severity] || 'default'}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>{alert.type || '-'}</TableCell>
                    <TableCell>{alert.message || '-'}</TableCell>
                    <TableCell>{alert.namespace || '-'}</TableCell>
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={() => handleViewDetails(alert)}
                      >
                        <Visibility />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle>Alert Details</DialogTitle>
        <DialogContent>
          {selectedAlert && (
            <>
              <DialogContentText>
                <strong>Type:</strong> {selectedAlert.type || '-'}
              </DialogContentText>
              <DialogContentText>
                <strong>Severity:</strong> {selectedAlert.severity || '-'}
              </DialogContentText>
              <DialogContentText>
                <strong>Timestamp:</strong>{' '}
                {selectedAlert.timestamp
                  ? format(new Date(selectedAlert.timestamp), 'yyyy-MM-dd HH:mm:ss')
                  : '-'}
              </DialogContentText>
              <DialogContentText>
                <strong>Namespace:</strong> {selectedAlert.namespace || '-'}
              </DialogContentText>
              <DialogContentText sx={{ mt: 2 }}>
                <strong>Message:</strong>
              </DialogContentText>
              <Paper sx={{ p: 2, mt: 1, bgcolor: 'grey.100' }}>
                <Typography variant="body2">{selectedAlert.message || '-'}</Typography>
              </Paper>
              {selectedAlert.flowData && (
                <>
                  <DialogContentText sx={{ mt: 2 }}>
                    <strong>Flow Data:</strong>
                  </DialogContentText>
                  <Paper sx={{ p: 2, mt: 1, bgcolor: 'grey.100' }}>
                    <pre style={{ fontSize: '12px', margin: 0 }}>
                      {JSON.stringify(selectedAlert.flowData, null, 2)}
                    </pre>
                  </Paper>
                </>
              )}
            </>
          )}
        </DialogContent>
      </Dialog>
    </Box>
  )
}

