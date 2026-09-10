import { useEffect, useRef } from 'react'

export default function BranchPicker({ branches, currentBranch, disabled, onChange }) {
  const pickerRef = useRef(null)
  const optionsRef = useRef(null)

  useEffect(() => {
    function dismiss(event) {
      if (!pickerRef.current.contains(event.target)) pickerRef.current.open = false
    }
    function closeOnResize() { pickerRef.current.open = false }
    document.addEventListener('pointerdown', dismiss)
    document.addEventListener('scroll', dismiss, true)
    window.addEventListener('resize', closeOnResize)
    return () => {
      document.removeEventListener('pointerdown', dismiss)
      document.removeEventListener('scroll', dismiss, true)
      window.removeEventListener('resize', closeOnResize)
    }
  }, [])

  function positionOptions() {
    const bounds = pickerRef.current.getBoundingClientRect()
    const below = window.innerHeight - bounds.bottom - 8
    const above = bounds.top - 8
    const openAbove = below < 260 && above > below
    Object.assign(optionsRef.current.style, {
      left: `${bounds.left}px`,
      width: `${bounds.width}px`,
      top: openAbove ? 'auto' : `${bounds.bottom + 4}px`,
      bottom: openAbove ? `${window.innerHeight - bounds.top + 4}px` : 'auto',
      maxHeight: `${Math.max(0, Math.min(260, openAbove ? above : below))}px`,
    })
  }
  const allBranches = [...new Set([...branches, currentBranch].filter(Boolean))].sort((a, b) => a.localeCompare(b))
  const rootBranches = allBranches.filter(branch => !branch.includes('/'))
  const folders = new Map()
  for (const branch of allBranches) {
    const slash = branch.indexOf('/')
    if (slash < 0) continue
    const folder = branch.slice(0, slash)
    if (!folders.has(folder)) folders.set(folder, [])
    folders.get(folder).push(branch)
  }

  function close(restoreFocus = false) {
    pickerRef.current.open = false
    if (restoreFocus) pickerRef.current.querySelector('summary').focus()
  }

  function branchButton(branch, label = branch) {
    return (
      <button
        key={branch}
        type="button"
        className="branch-option"
        title={branch}
        aria-pressed={branch === currentBranch}
        disabled={disabled}
        onClick={() => { close(true); onChange(branch) }}
      >
        <span>{label}</span>
        {branch === currentBranch && <span aria-hidden="true">✓</span>}
      </button>
    )
  }

  return (
    <details
      ref={pickerRef}
      className="branch-picker"
      onBlur={event => {
        if (!event.currentTarget.contains(event.relatedTarget)) close()
      }}
      onKeyDown={event => {
        if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true) }
      }}
    >
      <summary
        className="branch-trigger"
        title={currentBranch || 'Select branch'}
        aria-label={`Branch: ${currentBranch || 'Select branch'}`}
        aria-disabled={disabled}
        tabIndex={disabled ? -1 : 0}
        onClick={event => {
          if (disabled) event.preventDefault()
          else positionOptions()
        }}
      >{currentBranch || 'Select branch'}</summary>
      <div ref={optionsRef} className="branch-options">
        {allBranches.includes('develop') && branchButton('develop')}
        {rootBranches.filter(branch => branch !== 'develop').map(branch => branchButton(branch))}
        {[...folders].map(([folder, children]) => (
          <details className="branch-folder" key={folder}>
            <summary>{folder}/ <span className="branch-count">{children.length}</span></summary>
            <div className="branch-folder-children">
              {children.map(branch => branchButton(branch, branch.slice(folder.length + 1)))}
            </div>
          </details>
        ))}
        {allBranches.length === 0 && <div className="branch-empty">No branches available</div>}
      </div>
    </details>
  )
}
