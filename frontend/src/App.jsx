import { useEffect, useRef, useState, useCallback } from 'react'

const API = ''
const MAX_LOG_LINES = 2000
const EMPTY_LOGS = []

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

function SettingsPanel({ onClose, onManageEnvs }) {
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
      <div style={s.settingsDivider} />
      <label style={s.settingsLabel}>Environments</label>
      <p style={s.settingsHint}>Named sets of variables injected into APIs you start.</p>
      <button style={s.btn} onClick={onManageEnvs}>Manage environments…</button>
    </div>
  )
}

function RepoRow({ repo, selected, onSelect, onRemove, onReposChanged }) {
  const [branches, setBranches] = useState([])
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [pulling, setPulling] = useState(false)
  const [pullError, setPullError] = useState(null)
  const [switching, setSwitching] = useState(false)
  const [checkoutError, setCheckoutError] = useState(null)
  const [busy, setBusy] = useState('') // 'starting' | 'stopping' | ''
  const [processError, setProcessError] = useState('')

  const loadBranches = useCallback(() => {
    if (repo.pathError) return Promise.resolve()
    return fetch(`${API}/api/repos/${repo.id}/branches`)
      .then(r => r.json())
      .then(setBranches)
      .catch(() => {})
  }, [repo.id, repo.pathError])

  useEffect(() => { loadBranches() }, [loadBranches])

  const handleStartStop = async () => {
    if (repo.status === 'stopping') return
    setProcessError('')
    const isRunning = repo.status === 'running'
    setBusy(isRunning ? 'stopping' : 'starting')
    const res = await fetch(`${API}/api/repos/${repo.id}/${isRunning ? 'stop' : 'start'}`, { method: 'POST' })
    if (!res.ok) setProcessError(await res.text())
    else await onReposChanged()
    setBusy('')
  }

  const handlePull = async () => {
    setPulling(true)
    setPullError(null)
    const res = await fetch(`${API}/api/repos/${repo.id}/pull`, { method: 'POST' })
    if (res.ok) await onReposChanged()
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
    if (res.ok) await onReposChanged()
    else if (res.status === 409) setCheckoutError(await res.json())
    else setCheckoutError({ error: await res.text(), files: [] })
    setSwitching(false)
  }

  return (
    <li
      className={`api-row${selected ? ' api-row-selected' : ''}`}
      style={{ ...s.apiRow, ...(selected ? s.apiRowSelected : {}) }}
      onClick={() => onSelect(repo)}
    >
      <div style={s.apiRowTop}>
        <StatusDot status={repo.status} />
        <strong style={s.apiName}>{basename(repo.path)}</strong>
        <span style={s.statusLabel}>{repo.status}</span>
        {repo.status === 'running' && repo.envName && <span style={s.envBadge}>{repo.envName}</span>}
        {repo.reconnected && <span style={s.reconnectedBadge}>reconnected</span>}
        {repo.port > 0 && <span style={s.portBadge}>:{repo.port}</span>}
        <span style={s.apiRowSpacer} />
        <button
          style={s.removeBtn}
          onClick={e => {
            e.stopPropagation()
            const name = basename(repo.path)
            if (window.confirm(`Remove ${name} from API Manager?\n\nThe repository files will not be deleted.`)) {
              onRemove(repo.id)
            }
          }}
          title={`Remove ${basename(repo.path)}`}
          aria-label={`Remove ${basename(repo.path)}`}
        >✕</button>
        <button
          style={{ ...s.logsBtn, ...(selected ? s.logsBtnActive : {}) }}
          onClick={e => { e.stopPropagation(); onSelect(repo) }}
          aria-pressed={selected}
        >
          {selected ? 'Viewing logs' : 'Logs →'}
        </button>
      </div>
      <div style={s.apiPath} title={repo.path}>{repo.path}</div>

      {repo.pathError ? (
        <p style={s.rowError}>Path no longer exists on disk.</p>
      ) : (
        <>
          <div style={s.rowControls} onClick={e => e.stopPropagation()}>
            <select
              style={s.rowSelect}
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
            <button style={s.rowBtn} onClick={handleFetch} disabled={fetching}>
              {fetching ? <>Fetching <Spinner light={false} /></> : 'Fetch'}
            </button>
            <button style={s.rowBtn} onClick={handlePull} disabled={pulling}>
              {pulling ? <>Updating <Spinner light={false} /></> : 'Update'}
            </button>
            <button
              style={{ ...s.rowBtn, ...(repo.status === 'running' ? s.btnStop : s.btnStart) }}
              onClick={handleStartStop}
              disabled={busy !== '' || repo.status === 'stopping'}
            >
              {busy === 'starting' ? <>Starting <Spinner light /></>
                : busy === 'stopping' ? <>Stopping <Spinner light /></>
                : repo.status === 'running' ? 'Stop'
                : repo.status === 'stopping' ? <>Stopping <Spinner light /></> : 'Start'}
            </button>
          </div>

          {fetchError && <p style={s.rowError}>{fetchError}</p>}
          {processError && <p style={s.rowError}>{processError}</p>}

          {pullError && (
            <div style={s.rowDirtyBox}>
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
            <div style={s.rowDirtyBox}>
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

        </>
      )}
    </li>
  )
}

function ApiPane({ repos, selectedId, onSelect, onRemove, onReposChanged, environments, activeEnvId, onSwitchEnv, onManageEnvs, path, onPathChange, onAddRepo, addError }) {
  const [showSettings, setShowSettings] = useState(false)

  return (
    <aside className="api-pane" style={s.apiPane}>
      <div style={s.paneHeader}>
        <div>
          <span style={s.paneEyebrow}>Workspace</span>
          <h1 style={s.paneTitle}>APIs</h1>
        </div>
        <button style={s.gearBtn} onClick={() => setShowSettings(v => !v)} title="Settings">⚙</button>
      </div>
      <div style={s.envBar}>
        <span style={s.envBarLabel}>Environment</span>
        <select style={s.envSelect} value={activeEnvId} onChange={e => onSwitchEnv(e.target.value)} title="Applied to APIs you start">
          <option value="">None</option>
          {environments.map(e => <option key={e.id} value={e.id}>{e.name}</option>)}
        </select>
        <button style={s.envManageBtn} onClick={onManageEnvs} title="Manage environments">Manage</button>
      </div>
      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} onManageEnvs={onManageEnvs} />}
      <ul style={s.apiList}>
        {repos.length === 0 && <li style={s.empty}>No APIs registered yet.</li>}
        {repos.map(repo => (
          <RepoRow
            key={repo.id}
            repo={repo}
            selected={repo.id === selectedId}
            onSelect={onSelect}
            onRemove={onRemove}
            onReposChanged={onReposChanged}
          />
        ))}
      </ul>
      <div style={s.addBar}>
        <form onSubmit={onAddRepo} style={s.form}>
          <input
            style={s.input}
            type="text"
            placeholder="/path/to/repo"
            value={path}
            onChange={e => onPathChange(e.target.value)}
          />
          <button style={s.btn} type="submit">Add API</button>
        </form>
        {addError && <p style={s.errorText}>{addError}</p>}
      </div>
    </aside>
  )
}

function LogViewer({ repo, logSession, onSessionChange }) {
  const logBodyRef = useRef(null)
  const esRef = useRef(null)
  const pendingRef = useRef([])
  const runIdRef = useRef(logSession?.runId || '')
  const lastEventIdRef = useRef(logSession?.cursor || '')
  const endedRef = useRef(logSession?.ended || false)
  const retryRef = useRef(false)
  const repoId = repo?.id
  const repoStatus = repo?.status
  const reconnected = repo?.reconnected
  const logs = logSession?.lines || EMPTY_LOGS

  const updateSession = useCallback(updater => {
    if (repoId) onSessionChange(repoId, updater)
  }, [repoId, onSessionChange])

  const flushPending = useCallback(() => {
    if (pendingRef.current.length === 0) return
    const entries = pendingRef.current.splice(0)
    const runId = entries[entries.length - 1].runId
    const cursor = entries[entries.length - 1].cursor
    const lines = entries.map(entry => entry.line)
    updateSession(prev => {
      const previous = prev?.runId === runId ? prev : { runId, cursor: '', lines: [], ended: false, gap: false }
      return { ...previous, runId, cursor, ended: false, lines: [...previous.lines, ...lines].slice(-MAX_LOG_LINES) }
    })
  }, [updateSession])

  const clearDisplayedLogs = useCallback(() => {
    // Drop lines already received but not yet rendered as well. Persist the
    // latest cursor so changing views or reconnecting does not replay them.
    pendingRef.current = []
    updateSession(prev => ({
      ...prev,
      runId: runIdRef.current || prev.runId,
      cursor: lastEventIdRef.current || prev.cursor,
      lines: [],
      gap: false,
    }))
  }, [updateSession])

  useEffect(() => {
    retryRef.current = repoStatus === 'running' || repoStatus === 'stopping'
  }, [repoStatus])

  useEffect(() => {
    const id = setInterval(flushPending, 100)
    return () => {
      clearInterval(id)
      flushPending()
    }
  }, [flushPending])

  useEffect(() => {
    const el = logBodyRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [logs])

  useEffect(() => {
    if (!repoId || reconnected) {
      esRef.current?.close()
      esRef.current = null
      return
    }

    let active = true
    let es
    let retryTimer

    function connect() {
      endedRef.current = false
      const cursor = lastEventIdRef.current
      const suffix = cursor ? `?after=${encodeURIComponent(cursor)}` : ''
      es = new EventSource(`${API}/api/repos/${repoId}/logs${suffix}`)
      esRef.current = es
      es.onmessage = e => {
        let entry
        try { entry = JSON.parse(e.data) } catch { return }
        if (!entry.runId || typeof entry.line !== 'string') return
        if (entry.runId !== runIdRef.current) {
          pendingRef.current = []
          runIdRef.current = entry.runId
          endedRef.current = false
          updateSession(() => ({ runId: entry.runId, cursor: '', lines: [], ended: false, gap: false }))
        }
        if (e.lastEventId) lastEventIdRef.current = e.lastEventId
        const pending = pendingRef.current
        pending.push({ runId: entry.runId, cursor: e.lastEventId, line: entry.line })
        if (pending.length > MAX_LOG_LINES) {
          pending.splice(0, pending.length - MAX_LOG_LINES)
          updateSession(prev => ({ ...prev, runId: entry.runId, lines: [], gap: true }))
        }
      }
      const reset = e => {
        let detail
        try { detail = JSON.parse(e.data) } catch { return }
        pendingRef.current = []
        runIdRef.current = detail.runId || ''
        lastEventIdRef.current = ''
        endedRef.current = false
        updateSession(() => ({ runId: detail.runId || '', cursor: '', lines: [], ended: false, gap: e.type === 'retention-gap' }))
      }
      es.addEventListener('session-reset', reset)
      es.addEventListener('retention-gap', reset)
      es.addEventListener('session-end', e => {
        let detail = {}
        try { detail = JSON.parse(e.data) } catch { /* keep the current run */ }
        flushPending()
        endedRef.current = true
        updateSession(prev => ({ ...prev, runId: detail.runId || prev.runId, ended: true }))
        es.close()
      })
      es.onerror = () => {
        es.close()
        if (active && !endedRef.current && retryRef.current) retryTimer = setTimeout(connect, 2000)
      }
    }
    connect()

    return () => {
      active = false
      clearTimeout(retryTimer)
      es?.close()
      esRef.current = null
      flushPending()
    }
  }, [repoId, repoStatus, reconnected, flushPending, updateSession])

  if (!repo) {
    return <main className="log-viewer" style={{ ...s.logViewer, ...s.logViewerEmpty }}>Select an API to display its logs.</main>
  }

  const emptyMessage = reconnected
    ? 'Logs are unavailable for a process reconnected after API Manager restarted.'
    : repo.status === 'running' ? 'Waiting for output…' : 'No retained logs for this API.'

  return (
    <main className="log-viewer" style={s.logViewer}>
      <header style={s.logViewerHeader}>
        <div style={s.logViewerTitleRow}>
          <span style={s.logViewerEyebrow}>Logs</span>
          <StatusDot status={repo.status} />
          <span style={s.statusLabel}>{repo.status}</span>
          <button
            type="button"
            style={s.clearLogsBtn}
            onClick={clearDisplayedLogs}
            title="Clear displayed logs (backend history is preserved)"
          >Clear console</button>
        </div>
        <h2 style={s.logViewerTitle}>{basename(repo.path)}</h2>
        <div style={s.logViewerPath}>{repo.path}</div>
      </header>
      {logSession?.gap && <div style={s.logNotice}>Earlier output is no longer retained; showing the latest {MAX_LOG_LINES} lines.</div>}
      <div ref={logBodyRef} style={s.logBody}>
        {logs.length === 0
          ? <span style={s.logEmpty}>{emptyMessage}</span>
          : <pre style={s.logText}>{logs.join('\n')}</pre>}
      </div>
    </main>
  )
}

// Mirrors the backend parseEnvVars rules just to show a live count.
function countVars(vars) {
  return vars.split('\n').filter(line => {
    const t = line.trim()
    if (t === '' || t.startsWith('#')) return false
    const eq = t.indexOf('=')
    return eq > 0
  }).length
}

function EnvModal({ onClose }) {
  const [envs, setEnvs] = useState([])
  const [selectedId, setSelectedId] = useState(null) // env id, 'new', or null
  const [name, setName] = useState('')
  const [vars, setVars] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const load = useCallback((selectId) =>
    fetch(`${API}/api/environments`)
      .then(r => r.json())
      .then(data => {
        const list = data.environments || []
        setEnvs(list)
        if (selectId !== undefined) {
          const found = list.find(e => e.id === selectId)
          if (found) { setSelectedId(found.id); setName(found.name); setVars(found.vars) }
        }
      })
      .catch(() => {}), [])

  useEffect(() => { load() }, [load])

  const selectEnv = env => {
    setError('')
    setSelectedId(env.id)
    setName(env.name)
    setVars(env.vars)
  }

  const startNew = () => {
    setError('')
    setSelectedId('new')
    setName('')
    setVars('')
  }

  const save = async () => {
    setError('')
    if (!name.trim()) { setError('Name is required'); return }
    setSaving(true)
    const isNew = selectedId === 'new'
    const res = await fetch(
      isNew ? `${API}/api/environments` : `${API}/api/environments/${selectedId}`,
      {
        method: isNew ? 'POST' : 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), vars }),
      }
    )
    setSaving(false)
    if (!res.ok) { setError(await res.text()); return }
    if (isNew) {
      const created = await res.json()
      await load(created.id)
    } else {
      await load(selectedId)
    }
  }

  const remove = async () => {
    if (selectedId === 'new') { setSelectedId(null); return }
    const res = await fetch(`${API}/api/environments/${selectedId}`, { method: 'DELETE' })
    if (!res.ok) { setError(await res.text()); return }
    setSelectedId(null)
    setName('')
    setVars('')
    await load()
  }

  return (
    <div style={s.modalOverlay} onClick={onClose}>
      <div className="env-modal" style={s.modal} onClick={e => e.stopPropagation()}>
        <div style={s.modalHeader}>
          <span style={s.modalTitle}>Environments</span>
          <button style={s.closeBtn} onClick={onClose}>✕</button>
        </div>
        <div style={s.modalBody}>
          <div style={s.envList}>
            {envs.length === 0 && <p style={s.envListEmpty}>No environments yet.</p>}
            {envs.map(e => (
              <div
                key={e.id}
                style={{ ...s.envListItem, background: e.id === selectedId ? '#eef2ff' : 'transparent' }}
                onClick={() => selectEnv(e)}
              >{e.name}</div>
            ))}
            <button style={s.envNewBtn} onClick={startNew}>+ New environment</button>
          </div>
          <div style={s.envEditor}>
            {selectedId === null ? (
              <p style={s.envEditorHint}>Select an environment, or create one.</p>
            ) : (
              <>
                <label style={s.settingsLabel}>Name</label>
                <input
                  style={s.settingsInput}
                  type="text"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  placeholder="e.g. dev, staging, prod"
                  spellCheck={false}
                />
                <label style={s.settingsLabel}>Variables</label>
                <textarea
                  style={s.envTextarea}
                  value={vars}
                  onChange={e => setVars(e.target.value)}
                  placeholder={'# one KEY=VALUE per line\nSPRING_PROFILES_ACTIVE=dev\nDB_URL=jdbc:postgresql://localhost:5432/app'}
                  spellCheck={false}
                />
                <p style={s.settingsHint}>{countVars(vars)} variable{countVars(vars) === 1 ? '' : 's'} · {'#'} comments and blank lines ignored</p>
                {error && <p style={s.errorText}>{error}</p>}
                <div style={s.envEditorActions}>
                  <button style={{ ...s.btn, ...s.btnStart }} onClick={save} disabled={saving}>
                    {saving ? <>Saving <Spinner light /></> : 'Save'}
                  </button>
                  <button style={{ ...s.btn, ...s.btnDanger }} onClick={remove}>
                    {selectedId === 'new' ? 'Cancel' : 'Delete'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default function App() {
  const [repos, setRepos] = useState([])
  const [selected, setSelected] = useState(null)
  const [path, setPath] = useState('')
  const [addError, setAddError] = useState('')
  const [environments, setEnvironments] = useState([])
  const [activeEnvId, setActiveEnvId] = useState('')
  const [showEnvModal, setShowEnvModal] = useState(false)
  const [paneWidth, setPaneWidth] = useState(null)
  const [logSessions, setLogSessions] = useState({})
  const layoutRef = useRef(null)

  const fetchRepos = useCallback(() =>
    fetch(`${API}/api/repos`)
      .then(r => r.json())
      .then(data => {
        setRepos(data)
        setSelected(prev => prev ? (data.find(r => r.id === prev.id) ?? null) : null)
      }), [])

  const fetchEnvironments = useCallback(() =>
    fetch(`${API}/api/environments`)
      .then(r => r.json())
      .then(data => {
        setEnvironments(data.environments || [])
        setActiveEnvId(data.activeId || '')
      })
      .catch(() => {}), [])

  useEffect(() => {
    fetchRepos()
    const id = setInterval(fetchRepos, 3000)
    return () => clearInterval(id)
  }, [fetchRepos])

  useEffect(() => { fetchEnvironments() }, [fetchEnvironments])

  const switchEnv = async id => {
    const res = await fetch(`${API}/api/environments/active`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id }),
    })
    if (res.ok) setActiveEnvId(id)
  }

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

  const startResize = e => {
    if (!layoutRef.current || window.innerWidth <= 900) return
    e.preventDefault()
    const divider = e.currentTarget
    const bounds = layoutRef.current.getBoundingClientRect()
    const minLeft = Math.min(420, bounds.width - 360)
    const maxLeft = Math.max(minLeft, bounds.width - 360)

    const resize = event => {
      const width = Math.min(maxLeft, Math.max(minLeft, event.clientX - bounds.left))
      setPaneWidth(width)
    }
    const finish = () => {
      divider.removeEventListener('pointermove', resize)
      divider.removeEventListener('pointerup', finish)
      divider.removeEventListener('pointercancel', finish)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    divider.setPointerCapture(e.pointerId)
    divider.addEventListener('pointermove', resize)
    divider.addEventListener('pointerup', finish)
    divider.addEventListener('pointercancel', finish)
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  const updateLogSession = useCallback((id, updater) => {
    setLogSessions(all => ({
      ...all,
      [id]: updater(all[id] || { runId: '', cursor: '', lines: [], ended: false, gap: false }),
    }))
  }, [])

  const removeRepo = async id => {
    await fetch(`${API}/api/repos/${id}`, { method: 'DELETE' })
    setRepos(prev => prev.filter(r => r.id !== id))
    setSelected(prev => prev?.id === id ? null : prev)
    setLogSessions(prev => {
      const next = { ...prev }
      delete next[id]
      return next
    })
  }

  return (
    <div
      ref={layoutRef}
      className="app-layout"
      style={{ ...s.layout, '--api-pane-width': paneWidth ? `${paneWidth}px` : '40%' }}
    >
      <ApiPane
        repos={repos}
        selectedId={selected?.id}
        onSelect={setSelected}
        onRemove={removeRepo}
        onReposChanged={fetchRepos}
        environments={environments}
        activeEnvId={activeEnvId}
        onSwitchEnv={switchEnv}
        onManageEnvs={() => setShowEnvModal(true)}
        path={path}
        onPathChange={setPath}
        onAddRepo={addRepo}
        addError={addError}
      />
      <div
        className="pane-resizer"
        style={s.paneResizer}
        onPointerDown={startResize}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize API list and log viewer"
      ><span style={s.paneResizerHandle} /></div>
      {showEnvModal && (
        <EnvModal
          onClose={() => { setShowEnvModal(false); fetchEnvironments() }}
        />
      )}
      <LogViewer
        key={selected?.id ?? 'none'}
        repo={selected}
        logSession={selected ? logSessions[selected.id] : null}
        onSessionChange={updateLogSession}
      />
    </div>
  )
}

const s = {
  layout: { display: 'flex', height: '100vh', overflow: 'hidden', background: '#f8fafc', fontFamily: 'monospace', fontSize: 13, color: '#172033' },

  apiPane: { width: 'var(--api-pane-width)', minWidth: 420, maxWidth: 'calc(100% - 360px)', background: '#fff', display: 'flex', flexDirection: 'column', flexShrink: 0, overflow: 'hidden' },
  paneHeader: { minHeight: 72, padding: '14px 18px', borderBottom: '1px solid #e5e7eb', display: 'flex', alignItems: 'center', justifyContent: 'space-between' },
  paneEyebrow: { display: 'block', marginBottom: 2, color: '#94a3b8', fontSize: 10, fontWeight: 700, letterSpacing: 1, textTransform: 'uppercase' },
  paneTitle: { margin: 0, fontSize: 22, lineHeight: 1.1 },
  gearBtn: { width: 34, height: 34, background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 6, cursor: 'pointer', color: '#64748b', fontSize: 16, lineHeight: 1 },
  paneResizer: { position: 'relative', width: 9, margin: '0 -4px', flexShrink: 0, zIndex: 10, cursor: 'col-resize', touchAction: 'none', display: 'flex', alignItems: 'center', justifyContent: 'center' },
  paneResizerHandle: { display: 'block', width: 1, height: '100%', background: '#dbe2ea' },

  settingsPanel: { borderBottom: '1px solid #e5e7eb', padding: '14px 18px', background: '#f8fafc', maxHeight: '45vh', overflowY: 'auto' },
  settingsPanelHeader: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 },
  settingsPanelTitle: { fontSize: 11, fontWeight: 'bold', textTransform: 'uppercase', color: '#6b7280' },
  closeBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', fontSize: 14, padding: 4 },
  settingsLabel: { display: 'block', fontSize: 11, color: '#64748b', marginBottom: 5 },
  settingsInput: { width: '100%', boxSizing: 'border-box', padding: '8px 10px', fontFamily: 'monospace', fontSize: 12, border: '1px solid #cbd5e1', borderRadius: 5, marginBottom: 5, outline: 'none' },
  settingsHint: { fontSize: 10, color: '#94a3b8', margin: '0 0 10px', lineHeight: 1.5 },
  settingsDivider: { borderTop: '1px solid #e5e7eb', margin: '14px 0 12px' },
  btnSaved: { background: '#22c55e', border: '1px solid #16a34a', color: '#fff' },

  envBar: { display: 'flex', alignItems: 'center', gap: 8, padding: '10px 18px', borderBottom: '1px solid #e5e7eb', background: '#f8fafc' },
  envBarLabel: { fontSize: 10, fontWeight: 'bold', color: '#94a3b8', letterSpacing: 0.5, textTransform: 'uppercase' },
  envSelect: { flex: 1, minWidth: 0, fontFamily: 'monospace', fontSize: 12, padding: '6px 8px', border: '1px solid #cbd5e1', borderRadius: 5, background: '#fff' },
  envManageBtn: { background: '#fff', border: '1px solid #cbd5e1', borderRadius: 5, cursor: 'pointer', color: '#475569', fontFamily: 'monospace', fontSize: 11, padding: '6px 9px', lineHeight: 1 },
  envBadge: { fontSize: 10, color: '#4338ca', background: '#eef2ff', border: '1px solid #c7d2fe', borderRadius: 999, padding: '2px 7px' },

  modalOverlay: { position: 'fixed', inset: 0, background: 'rgba(15,23,42,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: 20 },
  modal: { width: '75vw', height: '80vh', minWidth: 'min(720px, 92vw)', minHeight: 'min(480px, 85vh)', maxWidth: '95vw', maxHeight: '92vh', resize: 'both', background: '#fff', borderRadius: 10, boxShadow: '0 18px 55px rgba(15,23,42,0.3)', display: 'flex', flexDirection: 'column', overflow: 'hidden' },
  modalHeader: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '16px 20px', borderBottom: '1px solid #e5e7eb' },
  modalTitle: { fontSize: 13, fontWeight: 'bold', textTransform: 'uppercase', color: '#374151', letterSpacing: 0.5 },
  modalBody: { display: 'flex', minHeight: 0, flex: 1 },
  envList: { width: '30%', minWidth: 210, borderRight: '1px solid #e5e7eb', padding: 12, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 4 },
  envListEmpty: { color: '#9ca3af', fontSize: 12, padding: '4px 8px', margin: 0 },
  envListItem: { padding: '9px 10px', borderRadius: 5, cursor: 'pointer', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  envNewBtn: { marginTop: 8, background: 'none', border: '1px dashed #cbd5e1', borderRadius: 5, cursor: 'pointer', color: '#64748b', fontSize: 12, padding: '9px 10px' },
  envEditor: { flex: 1, minWidth: 0, padding: 22, overflowY: 'auto', display: 'flex', flexDirection: 'column' },
  envEditorHint: { color: '#9ca3af', fontSize: 13, margin: 'auto', textAlign: 'center' },
  envTextarea: { width: '100%', boxSizing: 'border-box', flex: 1, minHeight: 300, resize: 'vertical', padding: '12px', fontFamily: 'monospace', fontSize: 13, lineHeight: 1.6, border: '1px solid #cbd5e1', borderRadius: 5, marginBottom: 5, outline: 'none', whiteSpace: 'pre-wrap', wordBreak: 'break-all' },
  envEditorActions: { display: 'flex', gap: 8, marginTop: 6 },
  btnDanger: { background: '#fff', border: '1px solid #fca5a5', color: '#dc2626' },

  empty: { padding: '28px 18px', color: '#94a3b8', fontSize: 12, margin: 0, textAlign: 'center' },
  apiList: { listStyle: 'none', padding: 0, margin: 0, overflowX: 'hidden', overflowY: 'auto', flex: 1, background: '#f8fafc' },
  apiRow: { minHeight: 142, padding: '16px 18px', cursor: 'pointer', background: '#fff', borderBottom: '1px solid #e5e7eb', borderLeft: '3px solid transparent', transition: 'background 0.15s ease, border-color 0.15s ease' },
  apiRowSelected: { background: '#f5f8ff', borderLeft: '3px solid #4f46e5' },
  apiRowTop: { display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 },
  apiName: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 15 },
  apiRowSpacer: { flex: 1 },
  apiPath: { margin: '7px 0 12px 16px', color: '#94a3b8', fontSize: 11, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  rowControls: { display: 'flex', alignItems: 'center', gap: 7, flexWrap: 'wrap', paddingLeft: 16 },
  rowSelect: { flex: '1 1 180px', minWidth: 140, maxWidth: 280, fontFamily: 'monospace', fontSize: 12, padding: '7px 9px', border: '1px solid #cbd5e1', borderRadius: 5, background: '#fff' },
  rowBtn: { padding: '7px 11px', cursor: 'pointer', fontFamily: 'monospace', fontSize: 12, border: '1px solid #cbd5e1', borderRadius: 5, background: '#fff', whiteSpace: 'nowrap' },
  logsBtn: { padding: '6px 9px', cursor: 'pointer', fontFamily: 'monospace', fontSize: 11, color: '#4338ca', background: '#fff', border: '1px solid #a5b4fc', borderRadius: 5, whiteSpace: 'nowrap' },
  logsBtnActive: { color: '#fff', background: '#4f46e5', border: '1px solid #4f46e5' },
  removeBtn: { background: 'none', border: 'none', cursor: 'pointer', color: '#cbd5e1', fontSize: 12, padding: 5, lineHeight: 1 },
  statusLabel: { fontSize: 12, color: '#6b7280' },
  btn: { padding: '7px 13px', cursor: 'pointer', fontFamily: 'monospace', fontSize: 12, border: '1px solid #cbd5e1', borderRadius: 5, background: '#fff' },
  btnStart: { background: '#22c55e', border: '1px solid #16a34a', color: '#fff' },
  btnStop: { background: '#ef4444', border: '1px solid #dc2626', color: '#fff' },
  portBadge: { fontSize: 11, color: '#64748b', background: '#f1f5f9', borderRadius: 999, padding: '2px 7px' },
  reconnectedBadge: { fontSize: 10, color: '#c2410c', background: '#fff7ed', border: '1px solid #fed7aa', borderRadius: 999, padding: '2px 7px' },

  rowError: { color: '#dc2626', margin: '9px 0 0 16px', fontSize: 11 },
  rowDirtyBox: { background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 5, padding: '9px 11px', margin: '10px 0 0 16px' },
  dirtyTitle: { margin: '0 0 6px', color: '#dc2626', fontWeight: 'bold', fontSize: 12 },
  dirtyList: { margin: 0, paddingLeft: 18, color: '#dc2626', fontSize: 12 },

  logViewer: { flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', padding: 18, background: '#eef2f7', overflow: 'hidden' },
  logViewerEmpty: { alignItems: 'center', justifyContent: 'center', color: '#94a3b8' },
  logViewerHeader: { flexShrink: 0, padding: '2px 2px 14px' },
  logViewerTitleRow: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 },
  logViewerEyebrow: { marginRight: 4, color: '#64748b', fontSize: 10, fontWeight: 700, letterSpacing: 1, textTransform: 'uppercase' },
  logViewerTitle: { margin: 0, fontSize: 20, lineHeight: 1.3 },
  logViewerPath: { marginTop: 3, color: '#94a3b8', fontSize: 11, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  logBody: { flex: 1, minHeight: 0, overflow: 'auto', padding: '14px 16px', background: '#0f172a', border: '1px solid #1e293b', borderRadius: 7, boxShadow: 'inset 0 1px 2px rgba(0,0,0,0.2)' },
  logText: { margin: 0, color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, lineHeight: '1.65', whiteSpace: 'pre-wrap', wordBreak: 'break-all' },
  logEmpty: { color: '#64748b', fontSize: 12 },
  clearLogsBtn: { marginLeft: 'auto', padding: '5px 9px', cursor: 'pointer', fontFamily: 'monospace', fontSize: 11, color: '#475569', background: '#fff', border: '1px solid #cbd5e1', borderRadius: 5 },
  logNotice: { flexShrink: 0, marginBottom: 8, padding: '7px 10px', color: '#92400e', background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 5, fontSize: 11 },

  addBar: { borderTop: '1px solid #e5e7eb', padding: '12px 18px', background: '#fff' },
  form: { display: 'flex', gap: 8 },
  input: { flex: 1, minWidth: 0, padding: '8px 10px', fontFamily: 'monospace', fontSize: 12, border: '1px solid #cbd5e1', borderRadius: 5, outline: 'none' },
  errorText: { color: '#ef4444', margin: '6px 0 0', fontSize: 12 },
}
