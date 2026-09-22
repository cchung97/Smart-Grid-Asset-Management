import { SearchX } from 'lucide-react'
import { Link } from 'react-router-dom'
import { EmptyState } from './ui/EmptyState'

export function NotFoundPanel() {
  return (
    <EmptyState icon={SearchX} title="Page not found" description="That address does not exist.">
      <Link to="/" className="text-sm font-medium text-primary underline">
        Back to the explorer
      </Link>
    </EmptyState>
  )
}
