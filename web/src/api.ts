export interface StageDetail {
  stage: string
  label: string
  state: 'completed' | 'in_progress' | 'pending' | 'failed' | 'cancelled'
  artifact_key?: string
}

export interface JobStagesResponse {
  job_id: string
  stage: string
  progress: number
  error?: string
  stages: StageDetail[]
}

export interface StylePreset {
  style: string
  prompt_directive: string
  negative_prompt: string
}

export interface SceneCard {
  scene_id: string
  index: number
  start_sec: number
  end_sec: number
  duration_sec: number
  transcript_text: string
  visual_type: string
  description: string
  location: string
  character_ids: string[]
  objects: string[]
  actions: string[]
  camera_motion: string
  transition: string
  prompt?: string
  image_key?: string
  regen_count?: number
  preview_url?: string
}

export interface StoryboardScenesResponse {
  job_id: string
  style: string
  title: string
  summary: string
  quality: { valid: boolean; issues?: string[] }
  scenes: SceneCard[]
}

export interface TranscriptSegment {
  text: string
  start: number
  end: number
  words?: { text: string; start: number; end: number }[]
}

export interface StoredTranscript {
  transcript: {
    provider: string
    model: string
    language: string
    text: string
    duration: number
    segments: TranscriptSegment[]
  }
  quality: { valid: boolean; warnings?: string[] }
}

export interface SubtitleStyle {
  animation: string
  font_name: string
  font_size: number
  primary_color: string
  highlight_color: string
  outline_color: string
  position: string
  max_words_per_line: number
}

export interface SubtitleTrack {
  style: SubtitleStyle
  cues: { index: number; start: number; end: number; text: string }[]
}

export interface CostEstimate {
  estimated_usd: number
  estimated_credits: number
  breakdown: {
    asr_usd: number
    llm_usd: number
    imagegen_usd: number
    compute_usd: number
  }
  assumptions: {
    duration_sec: number
    scene_count: number
    character_count: number
    quality: string
    asr_provider: string
    llm_provider: string
    imagegen_provider: string
    total_images: number
  }
}

async function reqJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `Request failed (${res.status})`)
  }
  return data as T
}

export const api = {
  health: () => reqJSON<{ status: string; service: string }>('/health'),

  listStyles: () => reqJSON<{ styles: StylePreset[] }>('/api/v1/styles'),

  estimateCost: (body: { duration_sec: number; quality?: string; scene_count?: number }) =>
    reqJSON<CostEstimate>('/api/v1/estimate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  uploadVideo: (file: File, onProgress?: (pct: number) => void): Promise<{ job_id: string; stage: string }> =>
    new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/api/v1/jobs/upload')
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress(Math.round((e.loaded / e.total) * 100))
        }
      }
      xhr.onload = () => {
        try {
          const data = JSON.parse(xhr.responseText || '{}')
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve(data)
          } else {
            reject(new Error(data.error || `Upload failed (${xhr.status})`))
          }
        } catch {
          reject(new Error(`Upload failed (${xhr.status})`))
        }
      }
      xhr.onerror = () => reject(new Error('Network error during upload'))
      const form = new FormData()
      form.append('video', file)
      xhr.send(form)
    }),

  getStages: (jobID: string) => reqJSON<JobStagesResponse>(`/api/v1/jobs/${encodeURIComponent(jobID)}/stages`),

  cancelJob: (jobID: string) =>
    reqJSON<{ job_id: string; status: string }>(`/api/v1/jobs/${encodeURIComponent(jobID)}`, { method: 'DELETE' }),

  getJobCost: (jobID: string) =>
    reqJSON<{ job_id: string; is_actual: boolean; estimate: CostEstimate }>(
      `/api/v1/jobs/${encodeURIComponent(jobID)}/cost`,
    ),

  getScenes: (jobID: string) =>
    reqJSON<StoryboardScenesResponse>(`/api/v1/jobs/${encodeURIComponent(jobID)}/storyboard/scenes`),

  patchScene: (
    jobID: string,
    sceneID: string,
    patch: Partial<{
      description: string
      duration_sec: number
      visual_type: string
      camera_motion: string
      transition: string
      actions: string[]
    }>,
  ) =>
    reqJSON<{ scene: SceneCard }>(
      `/api/v1/jobs/${encodeURIComponent(jobID)}/storyboard/scenes/${encodeURIComponent(sceneID)}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      },
    ),

  regenerateScene: (jobID: string, sceneID: string, prompt?: string) =>
    reqJSON<{ scene: SceneCard }>(
      `/api/v1/jobs/${encodeURIComponent(jobID)}/storyboard/scenes/${encodeURIComponent(sceneID)}/regenerate`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(prompt ? { prompt } : {}),
      },
    ),

  updateStoryboardStyle: (jobID: string, style: string) =>
    reqJSON<{ job_id: string; style: string }>(`/api/v1/jobs/${encodeURIComponent(jobID)}/storyboard/style`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ style }),
    }),

  getTranscript: (jobID: string) =>
    reqJSON<StoredTranscript>(`/api/v1/jobs/${encodeURIComponent(jobID)}/transcript`),

  updateTranscript: (jobID: string, transcript: StoredTranscript['transcript']) =>
    reqJSON<StoredTranscript>(`/api/v1/jobs/${encodeURIComponent(jobID)}/transcript`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(transcript),
    }),

  getSubtitles: (jobID: string) =>
    reqJSON<SubtitleTrack>(`/api/v1/jobs/${encodeURIComponent(jobID)}/subtitles`),

  updateSubtitleStyle: (jobID: string, style: SubtitleStyle) =>
    reqJSON<SubtitleTrack>(`/api/v1/jobs/${encodeURIComponent(jobID)}/subtitles/style`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(style),
    }),

  downloadUrl: (jobID: string) => `/api/v1/jobs/${encodeURIComponent(jobID)}/download`,
}
