import { useTranslation } from 'react-i18next';
import CodeBlock from '../../components/CodeBlock';

const claudeCodeSnippet = `# Set environment variables for Claude Code
export ANTHROPIC_BASE_URL=https://juezhou.org
export ANTHROPIC_API_KEY=your-relay-api-key

# Then run Claude Code as usual
claude`;

const openaiPythonSnippet = `import openai

client = openai.OpenAI(
    base_url="https://juezhou.org/v1",
    api_key="your-relay-api-key",
)

response = client.chat.completions.create(
    model="claude-opus-4-5",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)`;

const curlSnippet = `curl https://juezhou.org/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer your-relay-api-key" \\
  -d '{
    "model": "claude-opus-4-5",
    "messages": [
      {
        "role": "user",
        "content": "Hello!"
      }
    ]
  }'`;

export default function Docs() {
  const { t } = useTranslation();

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.docs')}</h2>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
        {/* Claude Code */}
        <section>
          <div style={{
            fontSize: 11,
            textTransform: 'uppercase',
            letterSpacing: 1,
            color: 'var(--text-muted)',
            marginBottom: 8,
          }}>
            {t('docs.claudeCodeSection')}
          </div>
          <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>
            {t('docs.claudeCodeDesc')}
          </p>
          <CodeBlock title="bash">{claudeCodeSnippet}</CodeBlock>
        </section>

        {/* OpenAI Python SDK */}
        <section>
          <div style={{
            fontSize: 11,
            textTransform: 'uppercase',
            letterSpacing: 1,
            color: 'var(--text-muted)',
            marginBottom: 8,
          }}>
            {t('docs.openaiSdkSection')}
          </div>
          <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>
            {t('docs.openaiSdkDesc')}
          </p>
          <CodeBlock title="python">{openaiPythonSnippet}</CodeBlock>
        </section>

        {/* curl */}
        <section>
          <div style={{
            fontSize: 11,
            textTransform: 'uppercase',
            letterSpacing: 1,
            color: 'var(--text-muted)',
            marginBottom: 8,
          }}>
            {t('docs.curlSection')}
          </div>
          <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>
            {t('docs.curlDesc')}{' '}
            <code style={{ fontFamily: 'var(--font-mono)' }}>/v1/chat/completions</code>
          </p>
          <CodeBlock title="curl">{curlSnippet}</CodeBlock>
        </section>
      </div>
    </div>
  );
}
