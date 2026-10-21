import { useId, type SelectHTMLAttributes } from 'react'

type SelectFieldProps = SelectHTMLAttributes<HTMLSelectElement> & {
  label: string
  options: { value: string; label: string }[]
}

export function SelectField({ label, options, className = '', ...selectProps }: SelectFieldProps) {
  const id = useId()
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <label htmlFor={id} className="text-sm font-medium text-slate-700">
        {label}
      </label>
      <select
        id={id}
        className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200"
        {...selectProps}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  )
}
