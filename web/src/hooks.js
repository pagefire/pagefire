import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { apiGet } from './api.js'

export function useApi(path) {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const hasFetched = useRef(false)

  const fetchData = useCallback(async () => {
    if (!hasFetched.current) setLoading(true)
    const result = await apiGet(path)
    setData(result.data)
    setError(result.error)
    setLoading(false)
    hasFetched.current = true
  }, [path])

  useEffect(() => {
    hasFetched.current = false
    fetchData()
  }, [fetchData])

  return { data, loading, error, refetch: fetchData }
}
