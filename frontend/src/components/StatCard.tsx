interface StatCardProps {
  label: string;
  value: string | number;
  color?: string;
}

export default function StatCard({ label, value, color = 'var(--accent-green)' }: StatCardProps) {
  return (
    <div style={{
      border: '1px solid var(--border-color)',
      padding: '16px 20px',
      backgroundColor: 'var(--bg-card)',
      fontFamily: 'var(--font-mono)',
    }}>
      <div style={{
        fontSize: 10,
        textTransform: 'uppercase',
        letterSpacing: 1,
        color: 'var(--text-muted)',
        marginBottom: 8,
      }}>
        {label}
      </div>
      <div style={{
        fontSize: 22,
        fontWeight: 700,
        color,
        lineHeight: 1,
      }}>
        {value}
      </div>
    </div>
  );
}
