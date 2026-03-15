import { useState } from 'preact/hooks'
import { useApi } from '../hooks.js'
import { apiPost, apiDelete } from '../api.js'
import { Modal } from '../components/modal.jsx'
import { TextInput, SelectInput } from '../components/form-field.jsx'
import { ConfirmDialog } from '../components/confirm-dialog.jsx'
import { useToast } from '../components/toast.jsx'
import { TimeAgo } from '../components/time-ago.jsx'
import { CopyButton } from '../components/copy-button.jsx'
import { useAuth } from '../auth.jsx'

const INTEGRATION_TYPES = [
  { value: 'generic', label: 'Generic Webhook' },
  { value: 'grafana', label: 'Grafana' },
  { value: 'prometheus', label: 'Prometheus Alertmanager' },
]

const CONDITION_FIELDS = [
  { value: 'summary', label: 'Summary' },
  { value: 'details', label: 'Details' },
  { value: 'source', label: 'Source' },
]

const MATCH_TYPES = [
  { value: 'contains', label: 'Contains' },
  { value: 'regex', label: 'Regex' },
]

export function ServiceDetail({ id }) {
  const { data: service, loading } = useApi(`/services/${id}`)
  const { data: integrationKeys, refetch: refetchKeys } = useApi(`/services/${id}/integration-keys`)
  const { data: routingRules, refetch: refetchRules } = useApi(`/services/${id}/routing-rules`)
  const { data: policies } = useApi('/escalation-policies')
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const toast = useToast()

  // Integration key state
  const [keyModalOpen, setKeyModalOpen] = useState(false)
  const [keyForm, setKeyForm] = useState({ name: '', type: 'generic' })
  const [keyErrors, setKeyErrors] = useState({})
  const [newKeySecret, setNewKeySecret] = useState(null)
  const [deleteKey, setDeleteKey] = useState(null)

  // Test integration key state
  const [testingKeyId, setTestingKeyId] = useState(null)

  // Routing rule state
  const [ruleModalOpen, setRuleModalOpen] = useState(false)
  const [ruleForm, setRuleForm] = useState({ condition_field: 'summary', condition_match_type: 'contains', condition_value: '', escalation_policy_id: '' })
  const [ruleErrors, setRuleErrors] = useState({})
  const [deleteRule, setDeleteRule] = useState(null)

  const policyOptions = (policies || []).map(p => ({ value: p.id, label: p.name }))
  const policyName = (pid) => {
    const p = (policies || []).find(p => p.id === pid)
    return p ? p.name : pid
  }

  // Integration key handlers
  const handleCreateKey = async () => {
    const errs = {}
    if (!keyForm.name.trim()) errs.name = 'Name is required'
    setKeyErrors(errs)
    if (Object.keys(errs).length > 0) return

    const { data, error } = await apiPost(`/services/${id}/integration-keys`, keyForm)
    if (error) {
      toast.error(error)
      return
    }
    setNewKeySecret(data.secret)
    toast.success('Integration key created')
    refetchKeys()
  }

  const handleDeleteKey = async () => {
    const { error } = await apiDelete(`/services/${id}/integration-keys/${deleteKey.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Integration key deleted')
      refetchKeys()
    }
    setDeleteKey(null)
  }

  const handleTestKey = async (keyId) => {
    setTestingKeyId(keyId)
    const { error } = await apiPost(`/services/${id}/integration-keys/${keyId}/test`, {})
    if (error) {
      toast.error('Test alert failed: ' + error)
    } else {
      toast.success('Test alert created successfully')
    }
    setTestingKeyId(null)
  }

  // Routing rule handlers
  const handleCreateRule = async () => {
    const errs = {}
    if (!ruleForm.condition_value.trim()) errs.condition_value = 'Condition value is required'
    if (!ruleForm.escalation_policy_id) errs.escalation_policy_id = 'Escalation policy is required'
    setRuleErrors(errs)
    if (Object.keys(errs).length > 0) return

    const { error } = await apiPost(`/services/${id}/routing-rules`, ruleForm)
    if (error) {
      toast.error(error)
      return
    }
    toast.success('Routing rule created')
    setRuleModalOpen(false)
    setRuleForm({ condition_field: 'summary', condition_match_type: 'contains', condition_value: '', escalation_policy_id: '' })
    refetchRules()
  }

  const handleDeleteRule = async () => {
    const { error } = await apiDelete(`/services/${id}/routing-rules/${deleteRule.id}`)
    if (error) {
      toast.error(error)
    } else {
      toast.success('Routing rule deleted')
      refetchRules()
    }
    setDeleteRule(null)
  }

  if (loading) return <div class="loading">Loading...</div>
  if (!service) return <div class="page"><p>Service not found</p></div>

  return (
    <div class="page">
      <div class="page-header">
        <div>
          <a href="/services" class="breadcrumb">Services</a>
          <h1>{service.name}</h1>
        </div>
      </div>

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Details</h3>
          <div class="detail-row">
            <span class="detail-label">Escalation Policy</span>
            <span>{policyName(service.escalation_policy_id)}</span>
          </div>
          {service.description && (
            <div class="detail-row">
              <span class="detail-label">Description</span>
              <span class="text-muted">{service.description}</span>
            </div>
          )}
          <div class="detail-row">
            <span class="detail-label">Created</span>
            <TimeAgo time={service.created_at} />
          </div>
        </div>

        {/* Integration Keys */}
        <div class="detail-card">
          <div class="card-header-row">
            <h3>Integration Keys</h3>
            {isAdmin && <button class="btn btn-primary btn-sm" onClick={() => { setKeyModalOpen(true); setKeyForm({ name: '', type: 'generic' }); setKeyErrors({}); setNewKeySecret(null) }}>
              Add Key
            </button>}
          </div>
          {!integrationKeys || integrationKeys.length === 0 ? (
            <p class="text-muted">No integration keys. Create one to receive alerts from external systems.</p>
          ) : (
            <div class="sub-list">
              {integrationKeys.map(k => (
                <div key={k.id} class="sub-list-item">
                  <div>
                    <span class="bold">{k.name}</span>
                    <span class="source-tag" style="margin-left: 8px">{k.type}</span>
                  </div>
                  <div class="sub-list-actions">
                    <span class="mono text-muted">{k.secret_prefix || '****'}</span>
                    <button
                      class="btn btn-secondary btn-sm"
                      onClick={() => handleTestKey(k.id)}
                      disabled={testingKeyId === k.id}
                      title="Send test alert"
                    >
                      {testingKeyId === k.id ? 'Sending...' : 'Test'}
                    </button>
                    {isAdmin && <button class="btn-icon btn-icon-danger" onClick={() => setDeleteKey(k)} title="Delete">&times;</button>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Routing Rules */}
      <div class="detail-card" style="margin-top: 20px">
        <div class="card-header-row">
          <h3>Routing Rules</h3>
          {isAdmin && <button class="btn btn-primary btn-sm" onClick={() => { setRuleModalOpen(true); setRuleErrors({}) }}>
            Add Rule
          </button>}
        </div>
        <p class="text-muted" style="margin-bottom: 12px; font-size: 13px">
          Route alerts to different escalation policies based on content. If no rule matches, the service's default policy is used.
        </p>
        {!routingRules || routingRules.length === 0 ? (
          <p class="text-muted">No routing rules configured.</p>
        ) : (
          <table class="data-table">
            <thead>
              <tr>
                <th>Field</th>
                <th>Match</th>
                <th>Value</th>
                <th>Route To</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {routingRules.map(r => (
                <tr key={r.id}>
                  <td>{r.condition_field}</td>
                  <td><span class="source-tag">{r.condition_match_type}</span></td>
                  <td class="mono">{r.condition_value}</td>
                  <td>{policyName(r.escalation_policy_id)}</td>
                  {isAdmin && (
                    <td class="row-actions">
                      <button class="btn-icon btn-icon-danger" onClick={() => setDeleteRule(r)} title="Delete">&times;</button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Integration Key Modal */}
      <Modal open={keyModalOpen} onClose={() => setKeyModalOpen(false)} title={newKeySecret ? 'Integration Key Created' : 'Add Integration Key'}>
        {newKeySecret ? (
          <div>
            <p class="text-muted" style="margin-bottom: 12px">Copy this secret now — it won't be shown again.</p>
            <div class="secret-display-row">
              <div class="secret-display"><code class="mono">{newKeySecret}</code></div>
              <CopyButton text={newKeySecret} />
            </div>
            <p class="text-muted" style="margin-top: 12px; font-size: 12px">
              Webhook URL: <code class="mono">/api/v1/integrations/{newKeySecret}/alerts</code>
            </p>
            <div class="form-actions">
              <button class="btn btn-primary" onClick={() => setKeyModalOpen(false)}>Done</button>
            </div>
          </div>
        ) : (
          <div>
            <TextInput label="Name" value={keyForm.name} onInput={(e) => setKeyForm(prev => ({ ...prev, name: e.target.value }))} error={keyErrors.name} placeholder="Grafana Production" />
            <SelectInput label="Type" value={keyForm.type} onChange={(e) => setKeyForm(prev => ({ ...prev, type: e.target.value }))} options={INTEGRATION_TYPES} />
            <div class="form-actions">
              <button class="btn btn-secondary" onClick={() => setKeyModalOpen(false)}>Cancel</button>
              <button class="btn btn-primary" onClick={handleCreateKey}>Create Key</button>
            </div>
          </div>
        )}
      </Modal>

      {/* Routing Rule Modal */}
      <Modal open={ruleModalOpen} onClose={() => setRuleModalOpen(false)} title="Add Routing Rule">
        <SelectInput label="Field" value={ruleForm.condition_field} onChange={(e) => setRuleForm(prev => ({ ...prev, condition_field: e.target.value }))} options={CONDITION_FIELDS} />
        <SelectInput label="Match Type" value={ruleForm.condition_match_type} onChange={(e) => setRuleForm(prev => ({ ...prev, condition_match_type: e.target.value }))} options={MATCH_TYPES} />
        <TextInput label="Value" value={ruleForm.condition_value} onInput={(e) => setRuleForm(prev => ({ ...prev, condition_value: e.target.value }))} error={ruleErrors.condition_value} placeholder={ruleForm.condition_match_type === 'regex' ? '.*critical.*' : 'database'} />
        <SelectInput label="Route To" value={ruleForm.escalation_policy_id} onChange={(e) => setRuleForm(prev => ({ ...prev, escalation_policy_id: e.target.value }))} options={policyOptions} placeholder="Select escalation policy..." error={ruleErrors.escalation_policy_id} />
        <div class="form-actions">
          <button class="btn btn-secondary" onClick={() => setRuleModalOpen(false)}>Cancel</button>
          <button class="btn btn-primary" onClick={handleCreateRule}>Create Rule</button>
        </div>
      </Modal>

      {/* Delete confirmations */}
      <ConfirmDialog
        open={!!deleteKey}
        onClose={() => setDeleteKey(null)}
        onConfirm={handleDeleteKey}
        title="Delete Integration Key"
        message={`Are you sure you want to delete "${deleteKey?.name}"? External systems using this key will no longer be able to send alerts.`}
      />
      <ConfirmDialog
        open={!!deleteRule}
        onClose={() => setDeleteRule(null)}
        onConfirm={handleDeleteRule}
        title="Delete Routing Rule"
        message="Are you sure you want to delete this routing rule? Alerts will fall back to the service's default escalation policy."
      />
    </div>
  )
}
