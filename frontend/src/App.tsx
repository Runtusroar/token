import { ConfigProvider } from 'antd';
import theme from './styles/theme';

function App() {
  return (
    <ConfigProvider theme={theme}>
      <div style={{ padding: '2rem' }}>
        <h1>AI Token Relay</h1>
      </div>
    </ConfigProvider>
  );
}

export default App;
