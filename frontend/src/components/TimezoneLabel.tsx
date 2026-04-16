import { useEffect, useState } from 'react';

function getTimezoneInfo(): { name: string; offset: string } {
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const now = new Date();
  const offsetMin = -now.getTimezoneOffset();
  const sign = offsetMin >= 0 ? '+' : '-';
  const h = String(Math.floor(Math.abs(offsetMin) / 60)).padStart(2, '0');
  const m = String(Math.abs(offsetMin) % 60).padStart(2, '0');
  return { name: tz, offset: `UTC${sign}${h}:${m}` };
}

export default function TimezoneLabel() {
  const [info, setInfo] = useState<{ name: string; offset: string } | null>(null);

  useEffect(() => {
    setInfo(getTimezoneInfo());
  }, []);

  if (!info) return null;

  return (
    <span
      title={info.name}
      style={{
        fontSize: 10,
        color: 'var(--text-muted)',
        border: '1px solid var(--border-color)',
        padding: '1px 6px',
        fontFamily: 'var(--font-mono)',
      }}
    >
      {info.offset}
    </span>
  );
}
