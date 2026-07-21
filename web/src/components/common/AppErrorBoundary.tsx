import { Component, type ErrorInfo, type ReactNode } from "react";
import { t } from "../../i18n";

type Props = { children: ReactNode };
type State = { failed: boolean };

export class AppErrorBoundary extends Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Unhandled React render error", error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="page app-error-boundary" role="alert">
        <section className="panel">
          <h1 className="page__title">{t("common.unexpectedError")}</h1>
          <button className="btn btn--primary" type="button" onClick={() => window.location.reload()}>
            {t("common.retry")}
          </button>
        </section>
      </main>
    );
  }
}
