import { useEffect, useState } from "react";
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
  Skeleton,
  Fade,
} from "@mui/material";
import { Refresh, Download, Search } from "@mui/icons-material";
import { format } from "date-fns";
import { flowsAPI } from "../services/api";
import useStore from "../store/useStore";

export default function FlowViewer() {
  const {
    flows,
    setFlows,
    flowsLoading,
    setFlowsLoading,
    flowsError,
    setFlowsError,
  } = useStore();

  const [page, setPage] = useState(0);
  const [rowsPerPage] = useState(25);
  const [searchTerm, setSearchTerm] = useState("");
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [verdictFilter, setVerdictFilter] = useState("");
  const [totalFlows, setTotalFlows] = useState(0);
  const [isInitialLoad, setIsInitialLoad] = useState(true);

  // Load API when filters or page change
  useEffect(() => {
    loadFlows();
  }, [page, rowsPerPage, searchTerm, namespaceFilter, verdictFilter]);

  // ----------------- WebSocket listener --------------------
  useEffect(() => {
    const wsUrl = "ws://localhost:5001/api/v1/stream/flows";
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => console.log("WS connected");

    ws.onmessage = () => {
      console.log("🔔 New flow received → silent refresh");
      // Silent refresh: only refresh if on page 0, don't show loading
      if (page === 0) {
        loadFlows(true); // true = silent refresh
      }
    };

    ws.onerror = (e) => console.error("WebSocket error:", e);

    ws.onclose = () => console.warn("WS connection closed");

    return () => ws.close();
  }, [page]);

  // ----------------- Load Flows via REST API ----------------
  const loadFlows = async (silent = false) => {
    try {
      // Only show loading spinner if not silent refresh and not initial load
      if (!silent && !isInitialLoad) {
        setFlowsLoading(true);
      }

      const params = {
        page: page + 1,
        limit: rowsPerPage,
      };
      if (searchTerm) params.search = searchTerm;
      if (namespaceFilter) params.namespace = namespaceFilter;
      if (verdictFilter) params.verdict = verdictFilter;

      const res = await flowsAPI.getAll(params);
      const items = Array.isArray(res.data?.items) ? res.data.items : [];

      setFlows(items);
      setTotalFlows(res.data?.total || 0);
      setFlowsError(null);
      
      if (isInitialLoad) {
        setIsInitialLoad(false);
      }
    } catch (err) {
      console.error("Failed to load flows:", err);
      setFlows([]);
      setTotalFlows(0);
      setFlowsError(err.message);
      setIsInitialLoad(false);
    } finally {
      setFlowsLoading(false);
    }
  };

  // ----------------- Table Pagination -----------------
  const handleChangePage = (_, newPage) => setPage(newPage);

  // ----------------- CSV Export -----------------
  const handleExport = () => {
    if (!Array.isArray(flows)) return;

    const csv = [
      [
        "Source Pod",
        "Source IP",
        "Source Identity",
        "Destination Pod",
        "Destination IP",
        "Destination Identity",
        "Destination Port",
        "Traffic Direction",
        "Verdict",
        "TCP Flags",
        "Timestamp",
      ].join(","),
      ...flows.map((f) => {
        const ts = f.timestamp
          ? format(new Date(f.timestamp), "yyyy/MM/dd HH:mm:ss")
          : "";
        // Support both snake_case (from API) and camelCase
        const sourceIP = f.source_ip || f.sourceIP || "";
        const destinationIP = f.destination_ip || f.destinationIP || "";
        const destinationPort = f.destination_port || f.destinationPort || "";
        
        return [
          f.source?.name || "",
          sourceIP,
          f.source?.identity || "",
          f.destination?.name || "",
          destinationIP,
          f.destination?.identity || "",
          destinationPort,
          f.traffic_direction || "",
          f.verdict || "",
          f.tcp_flags || "",
          ts,
        ].join(",");
      }),
    ].join("\n");

    const blob = new Blob([csv], { type: "text/csv" });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `flows-${new Date().toISOString()}.csv`;
    a.click();
  };

  const verdictColors = {
    FORWARDED: "success",
    DROPPED: "error",
    ERROR: "warning",
  };

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" mb={3}>
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

      {flowsError && <Alert severity="error">{flowsError}</Alert>}

      {/* ----------------- Filters ----------------- */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box display="flex" gap={2} flexWrap="wrap">
          <TextField
            label="Search"
            size="small"
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value);
              setPage(0);
            }}
            InputProps={{
              startAdornment: (
                <Search sx={{ mr: 1, color: "text.secondary" }} />
              ),
            }}
            sx={{ minWidth: 200 }}
          />

          <FormControl size="small" sx={{ minWidth: 150 }}>
            <InputLabel>Namespace</InputLabel>
            <Select
              value={namespaceFilter}
              label="Namespace"
              onChange={(e) => {
                setNamespaceFilter(e.target.value);
                setPage(0);
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
                setVerdictFilter(e.target.value);
                setPage(0);
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

      {/* ----------------- Table ----------------- */}
      {isInitialLoad && flows.length === 0 ? (
        // Initial load: show skeleton
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
                <TableCell>Direction</TableCell>
                <TableCell>Verdict</TableCell>
                <TableCell>TCP</TableCell>
                <TableCell>Timestamp</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {[...Array(5)].map((_, idx) => (
                <TableRow key={idx}>
                  {[...Array(11)].map((_, cellIdx) => (
                    <TableCell key={cellIdx}>
                      <Skeleton variant="text" width="100%" />
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      ) : (
        <Fade in={!flowsLoading} timeout={300}>
          <Box>
            <TableContainer 
              component={Paper} 
              sx={{ 
                position: 'relative',
                opacity: flowsLoading ? 0.6 : 1,
                transition: 'opacity 0.3s ease-in-out'
              }}
            >
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
                    <TableCell>Direction</TableCell>
                    <TableCell>Verdict</TableCell>
                    <TableCell>TCP</TableCell>
                    <TableCell>Timestamp</TableCell>
                  </TableRow>
                </TableHead>

                <TableBody>
                  {flows.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} align="center" sx={{ py: 4 }}>
                        <Typography variant="body2" color="text.secondary">
                          No flows found
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    flows.map((f, idx) => {
                      // Support both snake_case (from API) and camelCase
                      const sourceIP = f.source_ip || f.sourceIP || "-";
                      const destinationIP = f.destination_ip || f.destinationIP || "-";
                      const destinationPort = f.destination_port || f.destinationPort || "-";
                      
                      return (
                        <TableRow key={f.id || idx} hover>
                          <TableCell>{f.source?.name || "-"}</TableCell>
                          <TableCell>{sourceIP}</TableCell>
                          <TableCell>{f.source?.identity || "-"}</TableCell>
                          <TableCell>{f.destination?.name || "-"}</TableCell>
                          <TableCell>{destinationIP}</TableCell>
                          <TableCell>{f.destination?.identity || "-"}</TableCell>
                          <TableCell>{destinationPort}</TableCell>
                          <TableCell>
                            <Chip
                              label={f.traffic_direction || "-"}
                              size="small"
                              variant="outlined"
                            />
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={f.verdict || "-"}
                              color={verdictColors[f.verdict] || "default"}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>{f.tcp_flags || "-"}</TableCell>
                          <TableCell>
                            {f.timestamp
                              ? format(new Date(f.timestamp), "yyyy/MM/dd HH:mm:ss")
                              : "-"}
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
              {flowsLoading && (
                <Box
                  sx={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    backgroundColor: 'rgba(255, 255, 255, 0.8)',
                    zIndex: 1,
                  }}
                >
                  <CircularProgress size={40} />
                </Box>
              )}
            </TableContainer>

            <TablePagination
              component="div"
              count={totalFlows}
              page={page}
              rowsPerPage={rowsPerPage}
              onPageChange={handleChangePage}
              rowsPerPageOptions={[25]}
            />
          </Box>
        </Fade>
      )}
    </Box>
  );
}
