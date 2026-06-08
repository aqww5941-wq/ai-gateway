import { Component, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex h-full items-center justify-center">
          <div className="text-center space-y-4 max-w-md">
            <AlertTriangle className="w-12 h-12 text-destructive mx-auto opacity-50" />
            <div>
              <h2 className="text-lg font-semibold text-foreground">Something went wrong</h2>
              <p className="text-sm text-muted-foreground mt-1">{this.state.error.message}</p>
            </div>
            <button
              onClick={() => this.setState({ error: null })}
              className="px-4 py-2 text-sm font-medium rounded-md bg-primary text-primary-foreground hover:bg-primary/90"
            >
              Try again
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
