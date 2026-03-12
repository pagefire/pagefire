export function EmptyState({ message = 'No items found', hint }) {
  return (
    <div class="empty-state">
      <p>{message}</p>
      {hint && <p class="empty-state-hint">{hint}</p>}
    </div>
  )
}
