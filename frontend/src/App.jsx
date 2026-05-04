import { useEffect, useRef, useState, useCallback } from 'react'

const API = 'http://localhost:8080'

function basename(path) {
  return path.split('/').filter(Boolean).pop() ?? path
}

function StatusDot({ status }) {
  return (
    <span style={{
      display: 'inline-block', width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
      background: status === 'running' ? '#22c55e' : '#d1d5db',
    }} />
  )
}

function Sidebar({ repos, selectedId, onSelect, onRemove }) {
  return (
    <aside style={s.sidebar}>
      <div style={s.sidebarHeader}>Repos</div>
      {repos.length === 0 && <p style={s.empty}>No repos registered.</p>}
      <ul style={s.list}>
        {repos.map(r => (
          <li
            key={r.id}
            style={{ ...s.item, background: r.id === selectedId ? '#f5f5f5' : 'transparent' }}
            onClick={() => onSelect(r)}
          >
            <div style={s.itemRow}>
              <StatusDot status={r.status} />
              <span style={s.repoName}>{basename(r.path)}</span>
              <button style={s.removeBtn} onClick={e => { e.stopPropagation(); onRemove(r.id) }}>✕</button>
            </div>
            {r.pathError
              ? <span style={s.branchLabel} style2={s.branchError}>path not found</span>
              : <span style={s.branchLabel}>{r.currentBranch || '—'}</span>}
          </li>
        ))}
      </ul>
    </aside>
  )
}

function DetailPanel({ repo, onBranchChanged, onStatusChanged }) {
  const [branches, setBranches] = useState([])
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [checkoutError, setCheckoutError] = useState(null)
  const [processError, setProcessError] = useState('')
  const [logs, setLogs] = useState([])
  const logsEndRef = useRef(null)
  const esRef = useRef(null)

  const loadBranches = useCallback(() => {
    if (!repo || repo.pathError) return
    fetch(`${API}/api/repos/${repo.id}/branches`)
      .then(r => r.json())
      .then(setBranches)
      .catch(() => {})
  }, [repo?.id, repo?.pathError])

  useEffect(() => {
    setBranches([])
    setFetchError('')
    setCheckoutError(null)
    setProcessError('')
    setLogs([])
    loadBranches()
  }, [repo?.id])

  // Connect SSE when running, disconnect when stopped
  useEffect(() => {
    if (!repo || repo.status !== 'running') {
      esRef.current?.close()
      esRef.current = null
      return
    }
    const es = new EventSource(`${API}/api/repos/${repo.id}/logs`)
    esRef.current = es
    es.onmessage = e => setLogs(prev => [...prev, e.data])
    es.onerror = () => { es.close(); esRef.current = null }
    return () => { es.close(); esRef.current = null }
  }, [repo?.id, repo?.status])

  // Auto-scroll to bottom as logs arrive
  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  if (!repo) {
    return <div style={{ ...s.detail, ...s.detailEmpty }}><span style={{ color: '#aaa' }}>Select a repo to manage it.</span></div>
  }

  const handleStartStop = async () => {
    setProcessError('')
    const isRunning = repo.status === 'running'
    if (!isRunning) setLogs([])
    const res = await fetch(`${API}/api/repos/${repo.id}/${isRunning ? 'stop' : 'start'}`, { method: 'POST' })
    if (!res.ok) setProcessError(await res.text())
    else onStatusChanged()
  }

  const handleFetch = async () => {
    setFetching(true)
    setFetchError('')
    const res = await fetch(`${API}/api/repos/${repo.id}/fetch`, { method: 'POST' })
    setFetching(false)
    if (!res.ok) setFetchError(await res.text())
    else loadBranches()
  }

  const handleCheckout = async branch => {
    if (branch === repo.currentBranch) return
    setCheckoutError(null)
    const res = await fetch(`${API}/api/repos/${repo.id}/checkout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ branch }),
    })
    if (res.ok) { onBranchChanged(); return }
    if (res.status === 409) setCheckoutError(await res.json())
    else setCheckoutError({ error: await res.text(), files: [] })
  }

  return (
    <div style={s.detail}>
      <div style={s.detailHeader}>
        <h2 style={s.detailTitle}>{basename(repo.path)}</h2>
        <StatusDot status={repo.status} />
        <span style={s.statusLabel}>{repo.status}</span>
      </div>
      <p style={s.detailPath}>{repo.path}</p>

      {repo.pathError ? (
        <p style={s.errorText}>Path no longer exists on disk.</p>
      ) : (
        <>
          <div style={s.controls}>
            <select style={s.select} value={repo.currentBranch || ''} onChange={e => handleCheckout(e.target.value)}>
              {repo.currentBranch && !branches.includes(repo.currentBranch) && (
                <option value={repo.currentBranch}>{repo.currentBranch}</option>
              )}
              {branches.map(b => <option key={b} value={b}>{b}</option>)}
            </select>
            <button style={s.btn} onClick={handleFetch} disabled={fetching}>
              {fetching ? 'Fetching…' : 'Fetch'}
            </button>
            <button
              style={{ ...s.btn, ...(repo.status === 'running' ? s.btnStop : s.btnStart) }}
              onClick={handleStartStop}
            >
              {repo.status === 'running' ? 'Stop' : 'Start'}
            </button>
            {repo.port > 0 && <span style={s.portBadge}>:{repo.port}</span>}
          </div>

          {fetchError && <p style={s.errorText}>{fetchError}</p>}
          {processError && <p style={s.errorText}>{processError}</p>}

          {checkoutError && (
            <div style={s.dirtyBox}>
              <p style={s.dirtyTitle}>
                {checkoutError.error === 'dirty'
                  ? 'Cannot switch branch — uncommitted changes:'
                  : checkoutError.error}
              </p>
              {checkoutError.files?.length > 0 && (
                <ul style={s.dirtyList}>
                  {checkoutError.files.map(f => <li key={f}>{f}</li>)}
                </ul>
              )}
            </div>
          )}

          <div style={s.logPanel}>
            <div style={s.logHeader}>Logs</div>
            <div style={s.logBody}>
              {logs.length === 0
                ? <span style={s.logEmpty}>{repo.status === 'running' ? 'Waiting for output…' : 'Start the service to see logs.'}</span>
                : logs.map((line, i) => <div key={i} style={s.logLine}>{line}</div>)
              }
              <div ref={logsEndRef} />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export default function App() {
  const [repos, setRepos] = useState([])
  const [selected, setSelected] = useState(null)
  const [path, setPath] = useState('')
  const [addError, setAddError] = useState('')

  const fetchRepos = useCallback(() =>
    fetch(`${API}/api/repos`)
      .then(r => r.json())
      .then(data => {
        setRepos(data)
        setSelected(prev => prev ? (data.find(r => r.id === prev.id) ?? null) : null)
      }), [])

  useEffect(() => {
    fetchRepos()
    const id = setInterval(fetchRepos, 3000)
    return () => clearInterval(id)
  }, [fetchRepos])

  const addRepo = async e => {
    e.preventDefault()
    setAddError('')
    const res = await fetch(`${API}/api/repos`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    })
    if (!res.ok) { setAddError(await res.text()); return }
    const repo = await res.json()
    setPath('')
    setRepos(prev => [...prev, repo])
    setSelected(repo)
  }

  const removeRepo = async id => {
    await fetch(`${API}/api/repos/${id}`, { method: 'DELETE' })
    setRepos(prev => prev.filter(r => r.id !== id))
    setSelected(prev => prev?.id === id ? null : prev)
  }

  return (
    <div style={s.layout}>
      <Sidebar repos={repos} selectedId={selected?.id} onSelect={setSelected} onRemove={removeRepo} />
      <div style={s.right}>
        <DetailPanel repo={selected} onBranchChanged={fetchRepos} onStatusChanged={fetchRepos} />
        <div style={s.addBar}>
          <form onSubmit={addRepo} style={s.form}>
            <input style={s.input} type="text" placeholder="/absolute/path/to/repo"
              value={path} onChange={e => setPath(e.target.value)} />
            <button style={s.btn} type="submit">Add repo</button>
          </form>
          {addError && <p style={s.errorText}>{addError}</p>}
        </div>
      </div>
    </div>
  )
}

const s = {
  layout: { display: 'flex', height: '100vh', fontFamily: 'monospace', fontSize: 13, color: '#1a1a1a' },

  sidebar: { width: 240, borderRight: '1px solid #e5e7eb', display: 'flex', flexDirection: 'column', flexShrink: 0 },
  sidebarHeader: { padding: '12px 16px', fontSize: 11, fontWeight: 'bold', textTransform: 'uppercase', color: '#9ca3af', borderBottom: '1px solid #e5e7eb' },
  empty: { padding: '12px 16px', color: '#aaa', fontSize: 12, margin: 0 },
  list: { listStyle: 'none', padding: 0, margin: 0, overflowY: 'auto', flex: 1 },
  item: { padding: '10px 16px', cursor: 'pointer', borderBottom: '1px solid #f3f4f6' },
  itemRow: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 2 },
  repoName: { fontWeight: 'bold', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  removeBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#d1d5db', fontSize: 11, padding: 0, lineHeight: 1 },
  branchLabel: { fontSize: 11, color: '#6b7280', paddingLeft: 16 },
  branchError: { color: '#ef4444' },

  right: { flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' },

  detail: { flex: 1, padding: '20px 28px', overflowY: 'auto', display: 'flex', flexDirection: 'column' },
  detailEmpty: { alignItems: 'center', justifyContent: 'center' },
  detailHeader: { display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 },
  detailTitle: { margin: 0, fontSize: 18, fontWeight: 'bold' },
  statusLabel: { fontSize: 12, color: '#6b7280' },
  detailPath: { margin: '0 0 16px', fontSize: 11, color: '#9ca3af' },

  controls: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 },
  select: { fontFamily: 'monospace', fontSize: 13, padding: '5px 8px', border: '1px solid #d1d5db', borderRadius: 4, minWidth: 160 },
  btn: { padding: '5px 14px', cursor: 'pointer', fontFamily: 'monospace', fontSize: 13, border: '1px solid #d1d5db', borderRadius: 4, background: '#fff' },
  btnStart: { background: '#22c55e', borderColor: '#16a34a', color: '#fff' },
  btnStop: { background: '#ef4444', borderColor: '#dc2626', color: '#fff' },
  portBadge: { marginLeft: 4, fontSize: 12, color: '#6b7280' },

  dirtyBox: { background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 4, padding: '10px 14px', marginBottom: 10 },
  dirtyTitle: { margin: '0 0 6px', color: '#dc2626', fontWeight: 'bold', fontSize: 12 },
  dirtyList: { margin: 0, paddingLeft: 18, color: '#dc2626', fontSize: 12 },

  logPanel: { flex: 1, display: 'flex', flexDirection: 'column', border: '1px solid #e5e7eb', borderRadius: 6, minHeight: 0, marginTop: 10 },
  logHeader: { padding: '6px 12px', fontSize: 11, fontWeight: 'bold', textTransform: 'uppercase', color: '#9ca3af', borderBottom: '1px solid #e5e7eb' },
  logBody: { flex: 1, overflowY: 'auto', padding: '8px 12px', background: '#0f172a', borderRadius: '0 0 6px 6px' },
  logLine: { color: '#e2e8f0', fontSize: 12, lineHeight: '1.6', whiteSpace: 'pre-wrap', wordBreak: 'break-all' },
  logEmpty: { color: '#475569', fontSize: 12 },

  addBar: { borderTop: '1px solid #e5e7eb', padding: '10px 28px' },
  form: { display: 'flex', gap: 8 },
  input: { flex: 1, padding: '6px 10px', fontFamily: 'monospace', fontSize: 13, border: '1px solid #d1d5db', borderRadius: 4, outline: 'none' },
  errorText: { color: '#ef4444', margin: '6px 0 0', fontSize: 12 },
}
