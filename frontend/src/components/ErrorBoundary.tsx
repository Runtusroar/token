import { Component } from 'react';
import type { ReactNode, ErrorInfo } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          fontFamily: 'monospace',
          flexDirection: 'column',
          gap: 16,
        }}>
          <div style={{ fontSize: 48 }}>:(</div>
          <div style={{ fontSize: 14, color: '#666' }}>Something went wrong</div>
          <pre style={{ fontSize: 11, color: '#999', maxWidth: 500, overflow: 'auto' }}>
            {this.state.error?.message}
          </pre>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: '6px 20px',
              fontFamily: 'monospace',
              fontSize: 12,
              border: '1px solid #ccc',
              background: '#fff',
              cursor: 'pointer',
            }}
          >
            reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
