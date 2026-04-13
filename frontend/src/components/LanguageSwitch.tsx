import { useTranslation } from 'react-i18next';

export default function LanguageSwitch() {
  const { i18n } = useTranslation();
  const isChinese = i18n.language === 'zh' || i18n.language?.startsWith('zh-');

  function toggle() {
    i18n.changeLanguage(isChinese ? 'en' : 'zh');
  }

  return (
    <button
      onClick={toggle}
      style={{
        background: 'none',
        border: '1px solid var(--border-color)',
        cursor: 'pointer',
        fontFamily: 'var(--font-mono)',
        fontSize: 11,
        padding: '2px 8px',
        color: 'var(--text-primary)',
        letterSpacing: 0.5,
      }}
      title={isChinese ? 'Switch to English' : '切换为中文'}
    >
      {isChinese ? 'EN' : '中文'}
    </button>
  );
}
