export function EmptyState({ message = 'No items found' }) {
  return (
    <div class="empty-state">
      <p>{message}</p>
    </div>
  )
}
