import { useState } from 'react';
import type { ReactNode } from 'react';

interface CodeBlockProps {
  children: ReactNode;
  title?: string;
}

export default function CodeBlock({ children, title }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  function handleCopy() {
    const text =
      typeof children === 'string'
        ? children
        : document.getElementById('codblock-content')?.innerText ?? '';
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  }

  return (
    <div style={{
      backgroundColor: 'var(--bg-code)',
      color: '#e0e0e0',
      fontFamily: 'var(--font-mono)',
      fontSize: 12,
      position: 'relative',
    }}>
      {title && (
        <div style={{
          padding: '6px 16px',
          borderBottom: '1px solid #333',
          color: '#888',
          fontSize: 11,
          letterSpacing: 0.5,
        }}>
          {title}
        </div>
      )}

      {/* Copy button */}
      <button
        onClick={handleCopy}
        style={{
          position: 'absolute',
          top: title ? 30 : 8,
          right: 10,
          background: 'none',
          border: '1px solid #444',
          color: copied ? 'var(--accent-green)' : '#888',
          cursor: 'pointer',
          fontFamily: 'var(--font-mono)',
          fontSize: 10,
          padding: '2px 8px',
          letterSpacing: 0.5,
          transition: 'color 0.15s',
        }}
      >
        {copied ? 'copied' : 'copy'}
      </button>

      <pre
        id="codblock-content"
        style={{
          padding: '14px 16px',
          margin: 0,
          overflowX: 'auto',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          paddingRight: 60,
        }}
      >
        {children}
      </pre>
    </div>
  );
}
