import { Component, type ReactNode } from 'react';

// Without a boundary, any render error unmounts the whole tree and leaves a
// blank white page. This shows a way out instead.
export default class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error) {
    console.error('Render error:', error);
  }

  reset = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.href = '/';
  };

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div style={{ fontFamily: 'system-ui, sans-serif', maxWidth: 480, margin: '15vh auto', padding: 16, textAlign: 'center' }}>
        <h2>Something went wrong</h2>
        <p style={{ color: '#666' }}>{this.state.error.message}</p>
        <button onClick={this.reset} style={{ padding: '8px 16px', cursor: 'pointer' }}>
          Reset and reload
        </button>
      </div>
    );
  }
}
