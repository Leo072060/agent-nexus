export function ViewAsControl({
  value,
  onChange,
  me,
}: {
  value: string
  onChange: (next: string) => void
  me?: string
}) {
  return (
    <div className="view-as">
      <label htmlFor="view-as-input">以此验证者身份查看</label>
      <input
        id="view-as-input"
        className="mono"
        value={value}
        placeholder={me ? `${me} (默认)` : '0x… (默认使用后端 /me 地址)'}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
      />
      {value && (
        <button type="button" onClick={() => onChange('')}>
          重置
        </button>
      )}
    </div>
  )
}
