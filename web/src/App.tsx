import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Clapperboard,
  DollarSign,
  Download,
  FileText,
  Film,
  Image as ImageIcon,
  Loader2,
  RefreshCw,
  Save,
  Sparkles,
  Subtitles,
  Upload,
  XCircle,
} from 'lucide-react'
import {
  api,
  CostEstimate,
  JobStagesResponse,
  SceneCard,
  StoredTranscript,
  StylePreset,
  SubtitleStyle,
} from './api'

type TabKey = 'storyboard' | 'transcript' | 'subtitles' | 'export'

const ANIMATION_TEMPLATES = [
  'pop',
  'bounce',
  'fade',
  'slide',
  'scale',
  'typewriter',
  'kinetic',
  'karaoke',
]

const VISUAL_TYPES = [
  'character_animation',
  'environment_illustration',
  'object_animation',
  'animated_diagram',
  'chart_infographic',
  'kinetic_typography',
  'visual_metaphor',
  'static_motion',
]

export default function App() {
  const [apiHealthy, setApiHealthy] = useState<boolean | null>(null)
  const [jobID, setJobID] = useState<string>('')
  const [jobInput, setJobInput] = useState<string>('')
  const [tab, setTab] = useState<TabKey>('storyboard')
  const [error, setError] = useState<string>('')

  // Upload + estimator state
  const [uploadPct, setUploadPct] = useState<number | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const [estDuration, setEstDuration] = useState(60)
  const [estQuality, setEstQuality] = useState('1080p')
  const [estimate, setEstimate] = useState<CostEstimate | null>(null)

  // Active job state
  const [stages, setStages] = useState<JobStagesResponse | null>(null)
  const [styles, setStyles] = useState<StylePreset[]>([])
  const [selectedStyle, setSelectedStyle] = useState('2d_cartoon')
  const [scenes, setScenes] = useState<SceneCard[]>([])
  const [editingScene, setEditingScene] = useState<Record<string, { description: string; duration_sec: number; visual_type: string }>>({})
  const [busyScene, setBusyScene] = useState<string | null>(null)

  // Transcript + Subtitles + Cost
  const [transcriptDoc, setTranscriptDoc] = useState<StoredTranscript | null>(null)
  const [subStyle, setSubStyle] = useState<SubtitleStyle>({
    animation: 'pop',
    font_name: 'Arial',
    font_size: 48,
    primary_color: '#FFFFFF',
    highlight_color: '#FFD700',
    outline_color: '#000000',
    position: 'bottom',
    max_words_per_line: 4,
  })
  const [jobCost, setJobCost] = useState<{ is_actual: boolean; estimate: CostEstimate } | null>(null)

  useEffect(() => {
    api.health().then(() => setApiHealthy(true)).catch(() => setApiHealthy(false))
    api.listStyles().then((r) => setStyles(r.styles)).catch(() => {})
  }, [])

  useEffect(() => {
    api.estimateCost({ duration_sec: estDuration, quality: estQuality })
      .then(setEstimate)
      .catch(() => {})
  }, [estDuration, estQuality])

  const loadJobArtifacts = useCallback(async (id: string) => {
    try {
      const sb = await api.getScenes(id)
      setScenes(sb.scenes)
      setSelectedStyle(sb.style)
      const draft: Record<string, { description: string; duration_sec: number; visual_type: string }> = {}
      for (const sc of sb.scenes) {
        draft[sc.scene_id] = {
          description: sc.description,
          duration_sec: Number(sc.duration_sec.toFixed(2)),
          visual_type: sc.visual_type,
        }
      }
      setEditingScene(draft)
    } catch {
      // storyboard not ready yet
    }

    api.getTranscript(id).then(setTranscriptDoc).catch(() => {})
    api.getSubtitles(id).then((t) => setSubStyle(t.style)).catch(() => {})
    api.getJobCost(id).then(setJobCost).catch(() => {})
  }, [])

  useEffect(() => {
    if (!jobID) return
    let active = true

    const poll = async () => {
      try {
        const st = await api.getStages(jobID)
        if (!active) return
        setStages(st)
        if (['storyboard_ready', 'generating_images', 'images_ready', 'animating', 'composing', 'completed'].includes(st.stage)) {
          loadJobArtifacts(jobID)
        }
      } catch (e) {
        if (active) setError((e as Error).message)
      }
    }

    poll()
    const timer = setInterval(poll, 2500)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [jobID, loadJobArtifacts])

  const handleUpload = async (file: File) => {
    setError('')
    setUploadPct(0)
    try {
      const res = await api.uploadVideo(file, setUploadPct)
      setJobID(res.job_id)
      setJobInput(res.job_id)
      setUploadPct(null)
    } catch (e) {
      setError((e as Error).message)
      setUploadPct(null)
    }
  }

  const handleSaveScene = async (sceneID: string) => {
    const patch = editingScene[sceneID]
    if (!patch) return
    setBusyScene(sceneID)
    setError('')
    try {
      await api.patchScene(jobID, sceneID, patch)
      await loadJobArtifacts(jobID)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusyScene(null)
    }
  }

  const handleRegenerateScene = async (sceneID: string) => {
    setBusyScene(sceneID)
    setError('')
    try {
      await api.regenerateScene(jobID, sceneID)
      await loadJobArtifacts(jobID)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusyScene(null)
    }
  }

  const handleStyleChange = async (style: string) => {
    setSelectedStyle(style)
    if (!jobID) return
    try {
      await api.updateStoryboardStyle(jobID, style)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const handleSaveTranscript = async () => {
    if (!transcriptDoc || !jobID) return
    try {
      const updated = await api.updateTranscript(jobID, transcriptDoc.transcript)
      setTranscriptDoc(updated)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const handleSaveSubtitles = async () => {
    if (!jobID) return
    try {
      const updated = await api.updateSubtitleStyle(jobID, subStyle)
      setSubStyle(updated.style)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <span className="brand-badge">
            <Clapperboard size={20} color="#fff" />
          </span>
          Reel Studio
        </div>

        <div className="topbar-controls">
          <span className="pill">
            <span className={`dot ${apiHealthy ? 'ok' : 'err'}`} />
            {apiHealthy ? 'API Connected' : apiHealthy === false ? 'API Offline' : 'Checking...'}
          </span>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (jobInput.trim()) {
                setError('')
                setJobID(jobInput.trim())
              }
            }}
            style={{ display: 'flex', gap: 6 }}
          >
            <input
              className="input"
              placeholder="Load existing Job ID..."
              value={jobInput}
              onChange={(e) => setJobInput(e.target.value)}
            />
            <button className="btn" type="submit">Load</button>
          </form>

          {jobID && (
            <button
              className="btn"
              onClick={() => {
                setJobID('')
                setStages(null)
                setScenes([])
                setTranscriptDoc(null)
                setJobCost(null)
              }}
            >
              <Upload size={15} /> New Video
            </button>
          )}
        </div>
      </header>

      {error && <div className="alert-err">{error}</div>}

      {!jobID ? (
        <div className="grid-2">
          <div className="card">
            <h2 className="card-title"><Upload size={18} /> Upload Narration Video</h2>
            <div
              className={`dropzone ${dragOver ? 'drag' : ''}`}
              onClick={() => fileRef.current?.click()}
              onDragOver={(e) => {
                e.preventDefault()
                setDragOver(true)
              }}
              onDragLeave={() => setDragOver(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragOver(false)
                const file = e.dataTransfer.files[0]
                if (file) handleUpload(file)
              }}
            >
              <Film size={36} color="#818cf8" style={{ marginBottom: 10 }} />
              <div style={{ fontWeight: 600, marginBottom: 6 }}>
                Drop your MP4, MOV, WebM, or MKV here
              </div>
              <div className="muted">
                Streamed directly to Reel Engine — AI extracts narration, generates synced illustrations & subtitles.
              </div>
              <input
                ref={fileRef}
                type="file"
                accept=".mp4,.mov,.webm,.mkv,.m4v"
                hidden
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) handleUpload(f)
                }}
              />
            </div>

            {uploadPct !== null && (
              <div style={{ marginTop: 16 }}>
                <div className="row" style={{ marginBottom: 6 }}>
                  <span className="muted">Uploading video...</span>
                  <span className="muted">{uploadPct}%</span>
                </div>
                <div className="progress-bar">
                  <div className="progress-fill" style={{ width: `${uploadPct}%` }} />
                </div>
              </div>
            )}
          </div>

          <div className="card">
            <h2 className="card-title"><DollarSign size={18} /> Pre-Flight Cost Estimator</h2>
            <div style={{ display: 'grid', gap: 12 }}>
              <label className="muted">
                Video Duration (seconds): <strong>{estDuration}s</strong>
                <input
                  type="range"
                  min={10}
                  max={300}
                  step={5}
                  value={estDuration}
                  onChange={(e) => setEstDuration(Number(e.target.value))}
                  style={{ width: '100%', marginTop: 6 }}
                />
              </label>

              <label className="muted">
                Export Quality
                <select
                  className="select"
                  style={{ width: '100%', marginTop: 6 }}
                  value={estQuality}
                  onChange={(e) => setEstQuality(e.target.value)}
                >
                  <option value="720p">720p (Fast)</option>
                  <option value="1080p">1080p (Standard)</option>
                  <option value="4k">4K (High Bitrate)</option>
                </select>
              </label>

              {estimate && (
                <div style={{ background: 'var(--surface-2)', padding: 14, borderRadius: 10, border: '1px solid var(--border)' }}>
                  <div className="row" style={{ marginBottom: 8 }}>
                    <span>Estimated Total</span>
                    <strong>${estimate.estimated_usd.toFixed(4)} ({estimate.estimated_credits} credits)</strong>
                  </div>
                  <div className="muted">
                    ~{estimate.assumptions.scene_count} scenes + {estimate.assumptions.character_count} character sheets
                    ({estimate.assumptions.total_images} images via {estimate.assumptions.imagegen_provider})
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="card">
            <div className="row">
              <div>
                <div style={{ fontWeight: 700, fontSize: '1.05rem' }}>
                  Job <code>{jobID}</code>
                </div>
                <div className="muted">
                  Stage: <strong>{stages?.stage || 'loading'}</strong> ({stages?.progress ?? 0}%)
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <button className="btn" onClick={() => loadJobArtifacts(jobID)}>
                  <RefreshCw size={15} /> Refresh
                </button>
                {stages?.stage !== 'completed' && stages?.stage !== 'cancelled' && (
                  <button className="btn btn-danger" onClick={() => api.cancelJob(jobID)}>
                    <XCircle size={15} /> Cancel
                  </button>
                )}
              </div>
            </div>

            <div className="progress-bar" style={{ marginTop: 14 }}>
              <div className="progress-fill" style={{ width: `${stages?.progress ?? 5}%` }} />
            </div>

            {stages && (
              <div className="stages-grid">
                {stages.stages.map((s) => (
                  <div key={s.stage} className={`stage-item ${s.state}`}>
                    <div style={{ fontWeight: 600 }}>{s.label}</div>
                    <div className="muted" style={{ textTransform: 'capitalize' }}>{s.state.replace('_', ' ')}</div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="tabs">
            <button className={`tab ${tab === 'storyboard' ? 'active' : ''}`} onClick={() => setTab('storyboard')}>
              <ImageIcon size={16} /> Storyboard ({scenes.length})
            </button>
            <button className={`tab ${tab === 'transcript' ? 'active' : ''}`} onClick={() => setTab('transcript')}>
              <FileText size={16} /> Transcript
            </button>
            <button className={`tab ${tab === 'subtitles' ? 'active' : ''}`} onClick={() => setTab('subtitles')}>
              <Subtitles size={16} /> Subtitles
            </button>
            <button className={`tab ${tab === 'export' ? 'active' : ''}`} onClick={() => setTab('export')}>
              <Download size={16} /> Export & Cost
            </button>
          </div>

          {tab === 'storyboard' && (
            <div>
              <div className="card" style={{ marginBottom: 18 }}>
                <div className="row" style={{ flexWrap: 'wrap' }}>
                  <div>
                    <strong>Visual Style</strong>
                    <div className="muted">Applies across character sheets and scene illustrations</div>
                  </div>
                  <select className="select" value={selectedStyle} onChange={(e) => handleStyleChange(e.target.value)}>
                    {styles.map((st) => (
                      <option key={st.style} value={st.style}>
                        {st.style}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {scenes.length === 0 ? (
                <div className="card muted">Storyboard scenes will appear here once the AI Story Director finishes.</div>
              ) : (
                <div className="scenes-grid">
                  {scenes.map((sc) => {
                    const edit = editingScene[sc.scene_id] || {
                      description: sc.description,
                      duration_sec: sc.duration_sec,
                      visual_type: sc.visual_type,
                    }
                    const isBusy = busyScene === sc.scene_id
                    return (
                      <div key={sc.scene_id} className="scene-card">
                        <div className="scene-preview">
                          {sc.preview_url ? (
                            <img src={`${sc.preview_url}?v=${sc.regen_count ?? 0}`} alt={sc.description} />
                          ) : (
                            <span className="muted">Pending illustration...</span>
                          )}
                        </div>
                        <div className="scene-body">
                          <div className="row">
                            <strong>Scene #{sc.index}</strong>
                            <span className="muted">{sc.start_sec.toFixed(1)}s – {sc.end_sec.toFixed(1)}s</span>
                          </div>

                          <textarea
                            className="textarea"
                            value={edit.description}
                            onChange={(e) =>
                              setEditingScene((prev) => ({
                                ...prev,
                                [sc.scene_id]: { ...edit, description: e.target.value },
                              }))
                            }
                          />

                          <div className="row">
                            <select
                              className="select"
                              style={{ flex: 1 }}
                              value={edit.visual_type}
                              onChange={(e) =>
                                setEditingScene((prev) => ({
                                  ...prev,
                                  [sc.scene_id]: { ...edit, visual_type: e.target.value },
                                }))
                              }
                            >
                              {VISUAL_TYPES.map((vt) => (
                                <option key={vt} value={vt}>{vt}</option>
                              ))}
                            </select>

                            <input
                              className="input"
                              type="number"
                              step="0.5"
                              min="0.5"
                              style={{ width: 82 }}
                              value={edit.duration_sec}
                              onChange={(e) =>
                                setEditingScene((prev) => ({
                                  ...prev,
                                  [sc.scene_id]: { ...edit, duration_sec: Number(e.target.value) },
                                }))
                              }
                            />
                          </div>

                          <div className="row">
                            <button className="btn" disabled={isBusy} onClick={() => handleSaveScene(sc.scene_id)}>
                              <Save size={14} /> Save Patch
                            </button>
                            <button className="btn btn-primary" disabled={isBusy} onClick={() => handleRegenerateScene(sc.scene_id)}>
                              {isBusy ? <Loader2 size={14} /> : <Sparkles size={14} />} Regenerate
                            </button>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )}

          {tab === 'transcript' && (
            <div className="card">
              <div className="row" style={{ marginBottom: 14 }}>
                <h3 className="card-title" style={{ margin: 0 }}>Narration Transcript</h3>
                <button className="btn btn-primary" onClick={handleSaveTranscript} disabled={!transcriptDoc}>
                  <Save size={14} /> Save Transcript
                </button>
              </div>
              {!transcriptDoc ? (
                <div className="muted">Transcript will appear after ASR completes.</div>
              ) : (
                <div style={{ display: 'grid', gap: 10 }}>
                  {transcriptDoc.transcript.segments.map((seg, i) => (
                    <div key={i} className="row" style={{ alignItems: 'flex-start' }}>
                      <span className="muted" style={{ minWidth: 90, paddingTop: 8 }}>
                        {seg.start.toFixed(1)}s – {seg.end.toFixed(1)}s
                      </span>
                      <input
                        className="input"
                        style={{ flex: 1 }}
                        value={seg.text}
                        onChange={(e) => {
                          const next = [...transcriptDoc.transcript.segments]
                          next[i] = { ...seg, text: e.target.value }
                          setTranscriptDoc({
                            ...transcriptDoc,
                            transcript: { ...transcriptDoc.transcript, segments: next },
                          })
                        }}
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {tab === 'subtitles' && (
            <div className="card">
              <div className="row" style={{ marginBottom: 16 }}>
                <h3 className="card-title" style={{ margin: 0 }}>Subtitle Styling</h3>
                <button className="btn btn-primary" onClick={handleSaveSubtitles}>
                  <Save size={14} /> Apply Style
                </button>
              </div>
              <div className="grid-2">
                <label className="muted">
                  Animation Template
                  <select
                    className="select"
                    style={{ width: '100%', marginTop: 6 }}
                    value={subStyle.animation}
                    onChange={(e) => setSubStyle({ ...subStyle, animation: e.target.value })}
                  >
                    {ANIMATION_TEMPLATES.map((t) => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </select>
                </label>

                <label className="muted">
                  Position
                  <select
                    className="select"
                    style={{ width: '100%', marginTop: 6 }}
                    value={subStyle.position}
                    onChange={(e) => setSubStyle({ ...subStyle, position: e.target.value })}
                  >
                    <option value="bottom">Bottom</option>
                    <option value="center">Center</option>
                    <option value="top">Top</option>
                  </select>
                </label>

                <label className="muted">
                  Primary Color
                  <input
                    className="input"
                    type="color"
                    style={{ width: '100%', height: 40, marginTop: 6 }}
                    value={subStyle.primary_color}
                    onChange={(e) => setSubStyle({ ...subStyle, primary_color: e.target.value })}
                  />
                </label>

                <label className="muted">
                  Highlight Color
                  <input
                    className="input"
                    type="color"
                    style={{ width: '100%', height: 40, marginTop: 6 }}
                    value={subStyle.highlight_color}
                    onChange={(e) => setSubStyle({ ...subStyle, highlight_color: e.target.value })}
                  />
                </label>
              </div>
            </div>
          )}

          {tab === 'export' && (
            <div className="grid-2">
              <div className="card">
                <div className="row" style={{ marginBottom: 14 }}>
                  <h3 className="card-title" style={{ margin: 0 }}>Final Video</h3>
                  {stages?.stage === 'completed' && (
                    <a className="btn btn-primary" href={api.downloadUrl(jobID)} download>
                      <Download size={14} /> Download MP4
                    </a>
                  )}
                </div>
                {stages?.stage === 'completed' ? (
                  <video className="video-player" controls src={api.downloadUrl(jobID)} />
                ) : (
                  <div className="muted">Video player will appear once composition finishes.</div>
                )}
              </div>

              <div className="card">
                <h3 className="card-title"><DollarSign size={18} /> Job Cost Breakdown</h3>
                {jobCost ? (
                  <div style={{ display: 'grid', gap: 10 }}>
                    <div className="row">
                      <span className="muted">ASR</span>
                      <span>${jobCost.estimate.breakdown.asr_usd.toFixed(4)}</span>
                    </div>
                    <div className="row">
                      <span className="muted">LLM Storyboard</span>
                      <span>${jobCost.estimate.breakdown.llm_usd.toFixed(4)}</span>
                    </div>
                    <div className="row">
                      <span className="muted">Image Generation</span>
                      <span>${jobCost.estimate.breakdown.imagegen_usd.toFixed(4)}</span>
                    </div>
                    <div className="row">
                      <span className="muted">FFmpeg Compute</span>
                      <span>${jobCost.estimate.breakdown.compute_usd.toFixed(4)}</span>
                    </div>
                    <hr style={{ borderColor: 'var(--border)', width: '100%' }} />
                    <div className="row">
                      <strong>Total ({jobCost.is_actual ? 'Actual' : 'Projected'})</strong>
                      <strong>${jobCost.estimate.estimated_usd.toFixed(4)} ({jobCost.estimate.estimated_credits} credits)</strong>
                    </div>
                  </div>
                ) : (
                  <div className="muted">Cost breakdown loading...</div>
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
