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
  Switch,
  Chip,
  IconButton,
  CircularProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  FormControlLabel,
  MenuItem,
} from '@mui/material'
import { Refresh, Edit } from '@mui/icons-material'
import { rulesAPI } from '../services/api'
import useStore from '../store/useStore'

const severityColors = {
  CRITICAL: 'error',
  HIGH: 'warning',
  MEDIUM: 'info',
  LOW: 'default',
}

export default function RulesManagement() {
  const { rules, setRules, updateRule, rulesLoading, setRulesLoading, rulesError, setRulesError } = useStore()
  const [editDialogOpen, setEditDialogOpen] = useState(false)
  const [selectedRule, setSelectedRule] = useState(null)
  const [editForm, setEditForm] = useState({ enabled: false, severity: '', description: '' })

  useEffect(() => {
    loadRules()
  }, [])

  const loadRules = async () => {
    try {
      setRulesLoading(true)
      const response = await rulesAPI.getAll()
      setRules(response.data || [])
      setRulesError(null)
    } catch (err) {
      setRulesError(err.message)
      console.error('Failed to load rules:', err)
    } finally {
      setRulesLoading(false)
    }
  }

  const handleToggleRule = async (rule) => {
    try {
      await rulesAPI.update(rule.id || rule.name, { enabled: !rule.enabled })
      updateRule(rule.id || rule.name, { enabled: !rule.enabled })
    } catch (err) {
      setRulesError(err.message)
      console.error('Failed to update rule:', err)
    }
  }

  const handleEditClick = (rule) => {
    setSelectedRule(rule)
    setEditForm({
      enabled: rule.enabled,
      severity: rule.severity,
      description: rule.description,
    })
    setEditDialogOpen(true)
  }

  const handleSaveEdit = async () => {
    try {
      await rulesAPI.update(selectedRule.id || selectedRule.name, editForm)
      updateRule(selectedRule.id || selectedRule.name, editForm)
      setEditDialogOpen(false)
      setSelectedRule(null)
    } catch (err) {
      setRulesError(err.message)
      console.error('Failed to update rule:', err)
    }
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4">Rules Management</Typography>
        <IconButton onClick={loadRules} disabled={rulesLoading}>
          <Refresh />
        </IconButton>
      </Box>

      {rulesError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {rulesError}
        </Alert>
      )}

      {rulesLoading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Enabled</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Description</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rules.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <Typography variant="body2" color="textSecondary">
                      No rules found
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                rules.map((rule, index) => (
                  <TableRow key={rule.id || rule.name || index} hover>
                    <TableCell>
                      <Typography variant="body2" fontWeight="medium">
                        {rule.name}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={rule.enabled}
                        onChange={() => handleToggleRule(rule)}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={rule.severity || 'UNKNOWN'}
                        color={severityColors[rule.severity] || 'default'}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" noWrap sx={{ maxWidth: 400 }}>
                        {rule.description || '-'}
                      </Typography>
                    </TableCell>
                    <TableCell>{rule.type || '-'}</TableCell>
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={() => handleEditClick(rule)}
                      >
                        <Edit />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      <Dialog open={editDialogOpen} onClose={() => setEditDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Edit Rule</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <FormControlLabel
              control={
                <Switch
                  checked={editForm.enabled}
                  onChange={(e) => setEditForm({ ...editForm, enabled: e.target.checked })}
                />
              }
              label="Enabled"
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="Severity"
              value={editForm.severity}
              onChange={(e) => setEditForm({ ...editForm, severity: e.target.value })}
              select
              sx={{ mb: 2 }}
            >
              <MenuItem value="CRITICAL">CRITICAL</MenuItem>
              <MenuItem value="HIGH">HIGH</MenuItem>
              <MenuItem value="MEDIUM">MEDIUM</MenuItem>
              <MenuItem value="LOW">LOW</MenuItem>
            </TextField>
            <TextField
              fullWidth
              label="Description"
              value={editForm.description}
              onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
              multiline
              rows={4}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleSaveEdit} variant="contained">
            Save
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

