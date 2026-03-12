export function FormField({ label, error, children }) {
  return (
    <div class="form-field">
      {label && <label class="form-label">{label}</label>}
      {children}
      {error && <span class="form-error">{error}</span>}
    </div>
  )
}

export function TextInput({ label, error, ...props }) {
  return (
    <FormField label={label} error={error}>
      <input class={`form-control${error ? ' form-control-error' : ''}`} type="text" {...props} />
    </FormField>
  )
}

export function TextArea({ label, error, ...props }) {
  return (
    <FormField label={label} error={error}>
      <textarea class={`form-control${error ? ' form-control-error' : ''}`} rows={3} {...props} />
    </FormField>
  )
}

export function SelectInput({ label, error, options, placeholder, ...props }) {
  return (
    <FormField label={label} error={error}>
      <select class={`form-control${error ? ' form-control-error' : ''}`} {...props}>
        {placeholder && <option value="">{placeholder}</option>}
        {options.map(o => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </FormField>
  )
}
