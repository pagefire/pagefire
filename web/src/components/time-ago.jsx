const UNITS = [
  { label: 'y', seconds: 31536000 },
  { label: 'mo', seconds: 2592000 },
  { label: 'd', seconds: 86400 },
  { label: 'h', seconds: 3600 },
  { label: 'm', seconds: 60 },
  { label: 's', seconds: 1 },
]

function formatAbsolute(time) {
  const d = new Date(time)
  return d.toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export function TimeAgo({ time }) {
  if (!time) return <span class="time-ago">—</span>

  const seconds = Math.floor((Date.now() - new Date(time).getTime()) / 1000)
  const abs = formatAbsolute(time)

  if (seconds < 5) return <span class="time-ago" title={abs}>just now</span>

  for (const unit of UNITS) {
    const count = Math.floor(seconds / unit.seconds)
    if (count >= 1) {
      return <span class="time-ago" title={abs}>{count}{unit.label} ago</span>
    }
  }

  return <span class="time-ago" title={abs}>just now</span>
}
