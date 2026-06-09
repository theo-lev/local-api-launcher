import { useEffect, useRef, useState, useCallback } from 'react'

const API = ''

function basename(path) {
  return path.split(/[\\/]/).filter(Boolean).pop() ?? path
}

function Spinner({ light = false }) {
  return (
    <span
      className="spinner"
      style={{ borderColor: light ? 'rgba(255,255,255,0.4)' : '#d1d5db', borderTopColor: light ? '#fff' : '#6b7280' }}
    />
  )
}

function StatusDot({ status }) {
  return (
    <span style={{
      display: 'inline-block', width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
      background: status === 'running' ? '#22c55e' : '#d1d5db',
    }} />
  )
}

function SettingsPanel({ onClose }) {
  const [mavenPath, setMavenPath] = useState('')
  const [jdkPath, setJdkPath] = useState('')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch(`${API}/api/settings`)
      .then(r => r.json())
      .then(s => {
        setMavenPath(s.mavenPath || '')
        setJdkPath(s.jdkPath || '')
      })
      .catch(() => {})
  }, [])

  const save = async () => {
    setError('')
    const res = await fetch(`${API}/api/settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mavenPath, jdkPath }),
    })
    if (!res.ok) { setError('Failed to save'); return }
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div style={s.settingsPanel}>
      <div style={s.settingsPanelHeader}>
        <span style={s.settingsPanelTitle}>Settings</span>
        <button style={s.closeBtn} onClick={onClose}>✕</button>
      </div>
      <label style={s.settingsLabel}>Maven executable</label>
      <input
        style={s.settingsInput}
        type="text"
        value={mavenPath}
        onChange={e => setMavenPath(e.target.value)}
        placeholder={`leave empty to use mvn / mvn.cmd from PATH`}
        spellCheck={false}
      />
      <p style={s.settingsHint}>
        Full path to mvn (or mvn.cmd on Windows).<br />
        Example: <code>C:\apache-maven\bin\mvn.cmd</code>
      </p>
      <label style={s.settingsLabel}>JDK path (JAVA_HOME)</label>
      <input
        style={s.settingsInput}
        type="text"
        value={jdkPath}
        onChange={e => setJdkPath(e.target.value)}
        placeholder="leave empty to use system JAVA_HOME"
        spellCheck={false}
      />
      <p style={s.settingsHint}>
        Root directory of the JDK that has your certificates.<br />
        Example: <code>C:\Program Files\Eclipse Adoptium\jdk-17</code>
      </p>
      {error && <p style={s.errorText}>{error}</p>}
      <button style={{ ...s.btn, ...(saved ? s.btnSaved : {}) }} onClick={save}>
        {saved ? 'Saved!' : 'Save'}
      </button>
    </div>
  )
}

function Sidebar({ repos, selectedId, onSelect, onRemove }) {
  const [showSettings, setShowSettings] = useState(false)

  return (
    <aside style={s.sidebar}>
      <div style={s.sidebarHeader}>
        <span>Repos</span>
        <button
          style={s.gearBtn}
          onClick={() => setShowSettings(v => !v)}
          title="Settings"
        >⚙</button>
      </div>
      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} />}
      {repos.length === 0 && !showSettings && <p style={s.empty}>No repos registered.</p>}
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
              ? <span style={{ ...s.branchLabel, color: '#ef4444' }}>path not found</span>
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
  const [pulling, setPulling] = useState(false)
  const [pullError, setPullError] = useState(null)
  const [switching, setSwitching] = useState(false)
  const [checkoutError, setCheckoutError] = useState(null)
  const [busy, setBusy] = useState('') // 'starting' | 'stopping' | ''
  const [processError, setProcessError] = useState('')
  const [logs, setLogs] = useState([])
  const logBodyRef = useRef(null)
  const esRef = useRef(null)
  const pendingRef = useRef([])

  const loadBranches = useCallback(() => {
    if (!repo || repo.pathError) return Promise.resolve()
    return fetch(`${API}/api/repos/${repo.id}/branches`)
      .then(r => r.json())
      .then(setBranches)
      .catch(() => {})
  }, [repo?.id, repo?.pathError])

  // The component is keyed by repo id in App, so state is per-repo and
  // branches only need loading once per mount.
  useEffect(() => { loadBranches() }, [loadBranches])

  // Drain pending SSE lines into state every 100ms to avoid per-line re-renders
  useEffect(() => {
    const id = setInterval(() => {
      if (pendingRef.current.length === 0) return
      const lines = pendingRef.current.splice(0)
      setLogs(prev => {
        const next = [...prev, ...lines]
        return next.length > 2000 ? next.slice(-2000) : next
      })
    }, 100)
    return () => clearInterval(id)
  }, [])

  // Auto-scroll to bottom when logs update
  useEffect(() => {
    const el = logBodyRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [logs])

  // SSE connection with auto-reconnect on error
  useEffect(() => {
    if (!repo || repo.status !== 'running') {
      esRef.current?.close()
      esRef.current = null
      return
    }

    let active = true
    let es
    let retryTimer

    function connect() {
      es = new EventSource(`${API}/api/repos/${repo.id}/logs`)
      esRef.current = es
      es.onmessage = e => { pendingRef.current.push(e.data) }
      es.onerror = () => {
        es.close()
        if (active) retryTimer = setTimeout(connect, 2000)
      }
    }
    connect()

    return () => {
      active = false
      clearTimeout(retryTimer)
      es?.close()
      esRef.current = null
    }
  }, [repo?.id, repo?.status])

  if (!repo) {
    return <div style={{ ...s.detail, ...s.detailEmpty }}><span style={{ color: '#aaa' }}>Select a repo to manage it.</span></div>
  }

  const handleStartStop = async () => {
    setProcessError('')
    const isRunning = repo.status === 'running'
    setBusy(isRunning ? 'stopping' : 'starting')
    if (!isRunning) { setLogs([]); pendingRef.current = [] }
    const res = await fetch(`${API}/api/repos/${repo.id}/${isRunning ? 'stop' : 'start'}`, { method: 'POST' })
    if (!res.ok) setProcessError(await res.text())
    else await onStatusChanged() // keep the pending label until the new status is shown
    setBusy('')
  }

  const handlePull = async () => {
    setPulling(true)
    setPullError(null)
    const res = await fetch(`${API}/api/repos/${repo.id}/pull`, { method: 'POST' })
    if (res.ok) await onBranchChanged()
    else if (res.status === 409) setPullError(await res.json())
    else setPullError({ error: await res.text(), files: [] })
    setPulling(false)
  }

  const handleFetch = async () => {
    setFetching(true)
    setFetchError('')
    const res = await fetch(`${API}/api/repos/${repo.id}/fetch`, { method: 'POST' })
    if (!res.ok) setFetchError(await res.text())
    else await loadBranches()
    setFetching(false)
  }

  const handleCheckout = async branch => {
    if (branch === repo.currentBranch) return
    setSwitching(true)
    setCheckoutError(null)
    const res = await fetch(`${API}/api/repos/${repo.id}/checkout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ branch }),
    })
    if (res.ok) await onBranchChanged()
    else if (res.status === 409) setCheckoutError(await res.json())
    else setCheckoutError({ error: await res.text(), files: [] })
    setSwitching(false)
  }

  return (
    <div style={s.detail}>
      <div style={s.detailHeader}>
        <h2 style={s.detailTitle}>{basename(repo.path)}</h2>
        <StatusDot status={repo.status} />
        <span style={s.statusLabel}>{repo.status}</span>
        {repo.reconnected && <span style={s.reconnectedBadge}>reconnected</span>}
      </div>
      <p style={s.detailPath}>{repo.path}</p>

      {repo.pathError ? (
        <p style={s.errorText}>Path no longer exists on disk.</p>
      ) : (
        <>
          <div style={s.controls}>
            <select
              style={s.select}
              value={repo.currentBranch || ''}
              onChange={e => handleCheckout(e.target.value)}
              disabled={switching}
            >
              {repo.currentBranch && !branches.includes(repo.currentBranch) && (
                <option value={repo.currentBranch}>{repo.currentBranch}</option>
              )}
              {branches.map(b => <option key={b} value={b}>{b}</option>)}
            </select>
            {switching && <Spinner />}
            <button style={s.btn} onClick={handleFetch} disabled={fetching}>
              {fetching ? <>Fetching <Spinner light={false} /></> : 'Fetch'}
            </button>
            <button style={s.btn} onClick={handlePull} disabled={pulling}>
              {pulling ? <>Updating <Spinner light={false} /></> : 'Update'}
            </button>
            <button
              style={{ ...s.btn, ...(repo.status === 'running' ? s.btnStop : s.btnStart) }}
              onClick={handleStartStop}
              disabled={busy !== ''}
            >
              {busy === 'starting' ? <>Starting <Spinner light /></>
                : busy === 'stopping' ? <>Stopping <Spinner light /></>
                : repo.status === 'running' ? 'Stop' : 'Start'}
            </button>
            {repo.port > 0 && <span style={s.portBadge}>:{repo.port}</span>}
          </div>

          {fetchError && <p style={s.errorText}>{fetchError}</p>}
          {processError && <p style={s.errorText}>{processError}</p>}

          {pullError && (
            <div style={s.dirtyBox}>
              <p style={s.dirtyTitle}>
                {pullError.error === 'dirty'
                  ? 'Cannot pull — uncommitted changes:'
                  : pullError.error}
              </p>
              {pullError.files?.length > 0 && (
                <ul style={s.dirtyList}>
                  {pullError.files.map(f => <li key={f}>{f}</li>)}
                </ul>
              )}
            </div>
          )}

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
            <div ref={logBodyRef} style={s.logBody}>
              {logs.length === 0
                ? <span style={s.logEmpty}>{repo.status === 'running' ? 'Waiting for output…' : 'Start the service to see logs.'}</span>
                : logs.map((line, i) => <div key={i} style={s.logLine}>{line}</div>)
              }
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

  return (
    <div style={s.layout}>
      <Sidebar repos={repos} selectedId={selected?.id} onSelect={setSelected} onRemove={async id => {
        await fetch(`${API}/api/repos/${id}`, { method: 'DELETE' })
        setRepos(prev => prev.filter(r => r.id !== id))
        setSelected(prev => prev?.id === id ? null : prev)
      }} />
      <div style={s.right}>
        <DetailPanel key={selected?.id ?? 'none'} repo={selected} onBranchChanged={fetchRepos} onStatusChanged={fetchRepos} />
        <div style={s.addBar}>
          <form onSubmit={addRepo} style={s.form}>
            <input style={s.input} type="text" placeholder="/path/to/repo  or  C:\path\to\repo"
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
  sidebarHeader: { padding: '12px 16px', fontSize: 11, fontWeight: 'bold', textTransform: 'uppercase', color: '#9ca3af', borderBottom: '1px solid #e5e7eb', display: 'flex', alignItems: 'center', justifyContent: 'space-between' },
  gearBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#9ca3af', fontSize: 14, padding: 0, lineHeight: 1 },

  settingsPanel: { borderBottom: '1px solid #e5e7eb', padding: '12px 16px', background: '#fafafa' },
  settingsPanelHeader: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 },
  settingsPanelTitle: { fontSize: 11, fontWeight: 'bold', textTransform: 'uppercase', color: '#6b7280' },
  closeBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#9ca3af', fontSize: 11, padding: 0 },
  settingsLabel: { display: 'block', fontSize: 11, color: '#6b7280', marginBottom: 4 },
  settingsInput: { width: '100%', boxSizing: 'border-box', padding: '5px 8px', fontFamily: 'monospace', fontSize: 12, border: '1px solid #d1d5db', borderRadius: 4, marginBottom: 4, outline: 'none' },
  settingsHint: { fontSize: 10, color: '#9ca3af', margin: '0 0 8px', lineHeight: 1.5 },
  btnSaved: { background: '#22c55e', borderColor: '#16a34a', color: '#fff' },

  empty: { padding: '12px 16px', color: '#aaa', fontSize: 12, margin: 0 },
  list: { listStyle: 'none', padding: 0, margin: 0, overflowY: 'auto', flex: 1 },
  item: { padding: '10px 16px', cursor: 'pointer', borderBottom: '1px solid #f3f4f6' },
  itemRow: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 2 },
  repoName: { fontWeight: 'bold', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  removeBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#d1d5db', fontSize: 11, padding: 0, lineHeight: 1 },
  branchLabel: { fontSize: 11, color: '#6b7280', paddingLeft: 16 },

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
  reconnectedBadge: { fontSize: 11, color: '#f97316', background: '#fff7ed', border: '1px solid #fed7aa', borderRadius: 4, padding: '2px 6px' },

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
