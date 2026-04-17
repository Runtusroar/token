import { useTranslation } from 'react-i18next';
import CodeBlock from '../../components/CodeBlock';

const BASE_URL = 'https://api.juezhou.org';
const HOST = 'api.juezhou.org';

// --- Claude Code: install (macOS / Linux / WSL) ---
const ccInstallBashOfficial = `curl -fsSL https://claude.ai/install.sh | bash`;
const ccInstallBashMirror = `curl -fsSL https://${HOST}/install.sh | bash`;

// --- Claude Code: install (Windows PowerShell) ---
const ccInstallPwshOfficial = `irm https://claude.ai/install.ps1 | iex`;
const ccInstallPwshMirror = `irm https://${HOST}/install.ps1 | iex`;

// --- npm fallback ---
const ccInstallNpm = `# Optional: switch npm registry first (useful in mainland China)
npm config set registry https://registry.npmmirror.com

npm install -g @anthropic-ai/claude-code`;

// --- Claude Code: configure env vars ---
const ccConfigUnix = `# Add to ~/.bashrc or ~/.zshrc, then restart your terminal
export ANTHROPIC_BASE_URL=${BASE_URL}
export ANTHROPIC_AUTH_TOKEN=YOUR_KEY`;

const ccConfigWin = `# PowerShell — current session only
$env:ANTHROPIC_BASE_URL = "${BASE_URL}"
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_KEY"

# To persist across sessions (run once, then reopen terminal)
setx ANTHROPIC_BASE_URL "${BASE_URL}"
setx ANTHROPIC_AUTH_TOKEN "YOUR_KEY"`;

// --- Claude Code: verify ---
const ccVerify = `claude --version    # should print a version number
claude              # starts the interactive session`;

// --- OpenAI SDK / curl (unchanged) ---
const openaiPythonSnippet = `import openai

client = openai.OpenAI(
    base_url="${BASE_URL}/v1",
    api_key="your-relay-api-key",
)

response = client.chat.completions.create(
    model="claude-opus-4-6",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)`;

const curlSnippet = `curl ${BASE_URL}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer your-relay-api-key" \\
  -d '{
    "model": "claude-opus-4-6",
    "messages": [
      {
        "role": "user",
        "content": "Hello!"
      }
    ]
  }'`;

// -----------------------------------------------------------------------------

const sectionLabelStyle: React.CSSProperties = {
  fontSize: 11,
  textTransform: 'uppercase',
  letterSpacing: 1,
  color: 'var(--text-muted)',
  marginBottom: 8,
};

const stepHeaderStyle: React.CSSProperties = {
  fontSize: 13,
  fontWeight: 700,
  color: 'var(--text-primary)',
  margin: '20px 0 6px',
};

const subLabelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  margin: '12px 0 6px',
  fontWeight: 600,
};

const descStyle: React.CSSProperties = {
  fontSize: 12,
  color: 'var(--text-secondary)',
  marginBottom: 8,
  lineHeight: 1.6,
};

export default function Docs() {
  const { t } = useTranslation();

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.docs')}</h2>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
        {/* ------------------------------------------------------------------ */}
        {/* 01 — Claude Code (full walkthrough)                                */}
        {/* ------------------------------------------------------------------ */}
        <section>
          <div style={sectionLabelStyle}>{t('docs.cc.section')}</div>
          <p style={descStyle}>{t('docs.cc.intro')}</p>

          {/* Step 1 — install */}
          <h3 style={stepHeaderStyle}>{t('docs.cc.step1')}</h3>
          <p style={descStyle}>{t('docs.cc.step1Desc')}</p>

          <div style={subLabelStyle}>{t('docs.cc.macLinux')} — {t('docs.cc.official')}</div>
          <CodeBlock title="bash">{ccInstallBashOfficial}</CodeBlock>

          <div style={subLabelStyle}>{t('docs.cc.macLinux')} — {t('docs.cc.mirror')}</div>
          <CodeBlock title="bash">{ccInstallBashMirror}</CodeBlock>

          <div style={subLabelStyle}>{t('docs.cc.windows')} — {t('docs.cc.official')}</div>
          <CodeBlock title="powershell">{ccInstallPwshOfficial}</CodeBlock>

          <div style={subLabelStyle}>{t('docs.cc.windows')} — {t('docs.cc.mirror')}</div>
          <CodeBlock title="powershell">{ccInstallPwshMirror}</CodeBlock>

          <div style={subLabelStyle}>{t('docs.cc.npmFallback')}</div>
          <p style={descStyle}>{t('docs.cc.npmFallbackDesc')}</p>
          <CodeBlock title="bash">{ccInstallNpm}</CodeBlock>

          {/* Step 2 — configure */}
          <h3 style={stepHeaderStyle}>{t('docs.cc.step2')}</h3>
          <p style={descStyle}>
            {t('docs.cc.step2Desc')}{' '}
            <strong style={{ color: 'var(--text-primary)' }}>{t('docs.cc.step2KeyHint')}</strong>
          </p>

          <div style={subLabelStyle}>{t('docs.cc.macLinux')}</div>
          <CodeBlock title="bash">{ccConfigUnix}</CodeBlock>

          <div style={subLabelStyle}>{t('docs.cc.windows')}</div>
          <CodeBlock title="powershell">{ccConfigWin}</CodeBlock>

          {/* Step 3 — verify */}
          <h3 style={stepHeaderStyle}>{t('docs.cc.step3')}</h3>
          <p style={descStyle}>{t('docs.cc.step3Desc')}</p>
          <CodeBlock title="bash">{ccVerify}</CodeBlock>

          {/* FAQ */}
          <h3 style={stepHeaderStyle}>{t('docs.cc.faq')}</h3>
          <ul style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.7, paddingLeft: 18 }}>
            <li><strong>{t('docs.cc.faq1Q')}</strong> — {t('docs.cc.faq1A')}</li>
            <li><strong>{t('docs.cc.faq2Q')}</strong> — {t('docs.cc.faq2A')}</li>
            <li><strong>{t('docs.cc.faq3Q')}</strong> — {t('docs.cc.faq3A')}</li>
            <li><strong>{t('docs.cc.faq4Q')}</strong> — {t('docs.cc.faq4A')}</li>
          </ul>
        </section>

        {/* ------------------------------------------------------------------ */}
        {/* 02 — OpenAI Python SDK                                             */}
        {/* ------------------------------------------------------------------ */}
        <section>
          <div style={sectionLabelStyle}>{t('docs.openaiSdkSection')}</div>
          <p style={descStyle}>{t('docs.openaiSdkDesc')}</p>
          <CodeBlock title="python">{openaiPythonSnippet}</CodeBlock>
        </section>

        {/* ------------------------------------------------------------------ */}
        {/* 03 — curl                                                          */}
        {/* ------------------------------------------------------------------ */}
        <section>
          <div style={sectionLabelStyle}>{t('docs.curlSection')}</div>
          <p style={descStyle}>
            {t('docs.curlDesc')}{' '}
            <code style={{ fontFamily: 'var(--font-mono)' }}>/v1/chat/completions</code>
          </p>
          <CodeBlock title="curl">{curlSnippet}</CodeBlock>
        </section>
      </div>
    </div>
  );
}
