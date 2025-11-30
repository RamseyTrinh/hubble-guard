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
  TextField,
  IconButton,
  Chip,
  TablePagination,
  Button,
  CircularProgress,
  Alert,
  MenuItem,
  FormControl,
  InputLabel,
  Select,
} from '@mui/material'
import { Refresh, Download, Search } from '@mui/icons-material'
import { format } from 'date-fns'
import { flowsAPI } from '../services/api'
import useStore from '../store/useStore'

export default function FlowViewer() {
  const { flows, setFlows, flowsLoading, setFlowsLoading, flowsError, setFlowsError } = useStore()
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [searchTerm, setSearchTerm] = useState('')
  const [namespaceFilter, setNamespaceFilter] = useState('')
  const [verdictFilter, setVerdictFilter] = useState('')

  // Load flows when filters change
  useEffect(() => {
    loadFlows()
  }, [page, rowsPerPage, searchTerm, namespaceFilter, verdictFilter])

  // WebSocket connection - separate effect, only runs once on mount
  useEffect(() => {
    const wsUrl = 'ws://localhost:5001/api/v1/stream/flows'
    let ws = null
    let isConnected = false
    let reconnectTimeout = null
    let reconnectAttempts = 0
    let shouldReconnect = true
    const maxReconnectAttempts = 5
    
    const connect = () => {
      if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
        return
      }
      
      try {
        ws = new WebSocket(wsUrl)
        
        ws.onopen = () => {
          console.log('WebSocket connected')
          isConnected = true
          reconnectAttempts = 0
        }
        
        ws.onmessage = (event) => {
          if (!isConnected) return
          
          try {
            const data = JSON.parse(event.data)
            
            // Handle connection confirmation message
            if (data.type === 'connected') {
              console.log('WebSocket connection confirmed:', data.message)
              return
            }
            
            // Handle flow data - can be single flow or array of flows (batch)
            const flowsToAdd = Array.isArray(data) ? data : [data]
            
            setFlows((prevFlows) => {
              // Ensure prevFlows is always an array
              const currentFlows = Array.isArray(prevFlows) ? prevFlows : []
              
              // Filter out duplicates and add new flows
              const newFlows = flowsToAdd.filter(newFlow => {
                // Check if flow already exists (by ID or timestamp+IP)
                return !currentFlows.some(f => f.id === newFlow.id || 
                  (f.timestamp === newFlow.timestamp && f.sourceIP === newFlow.sourceIP))
              })
              
              if (newFlows.length === 0) return currentFlows
              
              // Add new flows at the beginning, keep max 1000 flows
              return [...newFlows, ...currentFlows].slice(0, 1000)
            })
          } catch (err) {
            console.error('Failed to parse WebSocket message:', err)
          }
        }
        
        ws.onerror = (error) => {
          console.error('WebSocket error:', error)
          console.error('WebSocket readyState:', ws.readyState)
          console.error('WebSocket URL:', wsUrl)
          isConnected = false
        }
        
        ws.onclose = (event) => {
          console.log('WebSocket connection closed', {
            code: event.code,
            reason: event.reason,
            wasClean: event.wasClean
          })
          isConnected = false
          
          // Attempt to reconnect if not a normal closure and should reconnect
          if (shouldReconnect && event.code !== 1000 && reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000)
            console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${maxReconnectAttempts})...`)
            reconnectTimeout = setTimeout(() => {
              connect()
            }, delay)
          } else if (reconnectAttempts >= maxReconnectAttempts) {
            console.error('Max reconnection attempts reached. Please refresh the page.')
          }
        }
      } catch (err) {
        console.error('Failed to create WebSocket:', err)
        isConnected = false
      }
    }
    
    connect()
    
    return () => {
      shouldReconnect = false
      isConnected = false
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout)
      }
      if (ws) {
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close(1000, 'Component unmounting')
        }
      }
    }
  }, []) // Empty dependency array - only run once on mount

  const loadFlows = async () => {
    try {
      setFlowsLoading(true)
      const params = {
        page: page + 1,
        limit: rowsPerPage,
      }
      if (searchTerm) params.search = searchTerm
      if (namespaceFilter) params.namespace = namespaceFilter
      if (verdictFilter) params.verdict = verdictFilter

      const response = await flowsAPI.getAll(params)
      const flowsData = response.data?.items || response.data || []
      // Ensure flows is always an array
      setFlows(Array.isArray(flowsData) ? flowsData : [])
      setFlowsError(null)
    } catch (err) {
      setFlowsError(err.message)
      console.error('Failed to load flows:', err)
      setFlows([]) // Ensure flows is always an array even on error
    } finally {
      setFlowsLoading(false)
    }
  }

  const handleChangePage = (event, newPage) => {
    setPage(newPage)
  }

  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10))
    setPage(0)
  }

  const handleExport = () => {
    const csv = [
      [
        'Source Pod', 'Source IP', 'Source Identity',
        'Destination Pod', 'Destination IP', 'Destination Identity',
        'Destination Port', 'L7 info', 'Traffic Direction',
        'Verdict', 'TCP Flags', 'Timestamp'
      ].join(','),
      ...flows.map((flow) => {
        const sourcePod = flow.source?.name || ''
        const sourceIP = flow.sourceIP || flow.source?.ip || flow.ip?.source || ''
        const sourceIdentity = flow.source?.identity || 
          (flow.source?.namespace && flow.source?.name 
            ? `${flow.source.namespace}/${flow.source.name}` 
            : flow.source?.namespace || '')
        const destPod = flow.destination?.name || ''
        const destIP = flow.destinationIP || flow.destination?.ip || flow.ip?.destination || ''
        const destIdentity = flow.destination?.identity || 
          (flow.destination?.namespace && flow.destination?.name 
            ? `${flow.destination.namespace}/${flow.destination.name}` 
            : flow.destination?.namespace || '')
        const destPort = flow.destinationPort || flow.destination?.port || flow.port || ''
        const l7Info = flow.l7_info || flow.l7?.type || ''
        const direction = flow.traffic_direction || flow.direction || ''
        const tcpFlags = flow.tcp_flags || flow.tcp_flags || ''
        const timestamp = flow.timestamp 
          ? format(new Date(flow.timestamp), 'yyyy/MM/dd HH:mm:ss')
          : ''
        
        return [
          sourcePod, sourceIP, sourceIdentity,
          destPod, destIP, destIdentity,
          destPort, l7Info, direction,
          flow.verdict || '', tcpFlags, timestamp
        ].join(',')
      }),
    ].join('\n')

    const blob = new Blob([csv], { type: 'text/csv' })
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `flows-${new Date().toISOString()}.csv`
    a.click()
  }

  const verdictColors = {
    FORWARDED: 'success',
    DROPPED: 'error',
    ERROR: 'warning',
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4">Flow Viewer</Typography>
        <Box>
          <Button
            startIcon={<Download />}
            onClick={handleExport}
            variant="outlined"
            sx={{ mr: 1 }}
          >
            Export
          </Button>
          <IconButton onClick={loadFlows} disabled={flowsLoading}>
            <Refresh />
          </IconButton>
        </Box>
      </Box>

      {flowsError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {flowsError}
        </Alert>
      )}

      <Paper sx={{ p: 2, mb: 2 }}>
        <Box display="flex" gap={2} alignItems="center" flexWrap="wrap">
          <TextField
            label="Search"
            variant="outlined"
            size="small"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyPress={(e) => {
              if (e.key === 'Enter') {
                setPage(0)
                loadFlows()
              }
            }}
            InputProps={{
              startAdornment: <Search sx={{ mr: 1, color: 'text.secondary' }} />,
            }}
            sx={{ flexGrow: 1, minWidth: 200 }}
          />
          <FormControl size="small" sx={{ minWidth: 150 }}>
            <InputLabel>Namespace</InputLabel>
            <Select
              value={namespaceFilter}
              label="Namespace"
              onChange={(e) => {
                setNamespaceFilter(e.target.value)
                setPage(0)
                loadFlows()
              }}
            >
              <MenuItem value="">All</MenuItem>
              <MenuItem value="default">default</MenuItem>
              <MenuItem value="kube-system">kube-system</MenuItem>
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 150 }}>
            <InputLabel>Verdict</InputLabel>
            <Select
              value={verdictFilter}
              label="Verdict"
              onChange={(e) => {
                setVerdictFilter(e.target.value)
                setPage(0)
                loadFlows()
              }}
            >
              <MenuItem value="">All</MenuItem>
              <MenuItem value="FORWARDED">Forwarded</MenuItem>
              <MenuItem value="DROPPED">Dropped</MenuItem>
              <MenuItem value="ERROR">Error</MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Paper>

      {flowsLoading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <>
          <TableContainer component={Paper}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>Source Pod</TableCell>
                  <TableCell>Source IP</TableCell>
                  <TableCell>Source Identity</TableCell>
                  <TableCell>Destination Pod</TableCell>
                  <TableCell>Destination IP</TableCell>
                  <TableCell>Destination Identity</TableCell>
                  <TableCell>Destination Port</TableCell>
                  <TableCell>L7 info</TableCell>
                  <TableCell>Traffic Direction</TableCell>
                  <TableCell>Verdict</TableCell>
                  <TableCell>TCP Flags</TableCell>
                  <TableCell>Timestamp</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {!Array.isArray(flows) || flows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={12} align="center">
                      <Typography variant="body2" color="textSecondary">
                        No flows found
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  flows.map((flow, index) => {
                    const sourcePod = flow.source?.name || '-'
                    const sourceIP = flow.sourceIP || flow.source?.ip || flow.ip?.source || '-'
                    const sourceIdentity = flow.source?.identity || 
                      (flow.source?.namespace && flow.source?.name 
                        ? `${flow.source.namespace}/${flow.source.name}` 
                        : flow.source?.namespace || '-')
                    const destPod = flow.destination?.name || '-'
                    const destIP = flow.destinationIP || flow.destination?.ip || flow.ip?.destination || '-'
                    const destIdentity = flow.destination?.identity || 
                      (flow.destination?.namespace && flow.destination?.name 
                        ? `${flow.destination.namespace}/${flow.destination.name}` 
                        : flow.destination?.namespace || '-')
                    const destPort = flow.destinationPort || flow.destination?.port || flow.port || '-'
                    const l7Info = flow.l7_info || flow.l7?.type || '-'
                    const direction = flow.traffic_direction || flow.direction || '-'
                    const tcpFlags = flow.tcp_flags || flow.tcp_flags || '-'
                    
                    return (
                      <TableRow key={flow.id || index} hover>
                        <TableCell>{sourcePod}</TableCell>
                        <TableCell>{sourceIP}</TableCell>
                        <TableCell>{sourceIdentity}</TableCell>
                        <TableCell>{destPod}</TableCell>
                        <TableCell>{destIP}</TableCell>
                        <TableCell>{destIdentity}</TableCell>
                        <TableCell>{destPort}</TableCell>
                        <TableCell>{l7Info}</TableCell>
                        <TableCell>
                          <Chip
                            label={direction}
                            size="small"
                            color={direction === 'egress' ? 'primary' : direction === 'ingress' ? 'secondary' : 'default'}
                            variant="outlined"
                          />
                        </TableCell>
                        <TableCell>
                          <Chip
                            label={flow.verdict || '-'}
                            color={verdictColors[flow.verdict] || 'default'}
                            size="small"
                          />
                        </TableCell>
                        <TableCell>{tcpFlags}</TableCell>
                        <TableCell>
                          {flow.timestamp
                            ? format(new Date(flow.timestamp), 'yyyy/MM/dd HH:mm:ss')
                            : '-'}
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </TableContainer>
          <TablePagination
            component="div"
            count={flows.length}
            page={page}
            onPageChange={handleChangePage}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleChangeRowsPerPage}
            rowsPerPageOptions={[10, 25, 50, 100]}
          />
        </>
      )}
    </Box>
  )
}

