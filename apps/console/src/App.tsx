import { ApiError, type DependencyCheck, type Info } from "@tigerbeetle-platform/api-client";
import { useCallback, useEffect, useState } from "react";
import { api } from "./api";

const REFRESH_MS = 5_000;

type State =
  | { kind: "loading" }
  | { kind: "ready"; info: Info; fetchedAt: Date }
  | { kind: "error"; message: string; requestId?: string };

export function App() {
  const [state, setState] = useState<State>({ kind: "loading" });

  const refresh = useCallback(async () => {
    try {
      const info = await api.getInfo();
      setState({ kind: "ready", info, fetchedAt: new Date() });
    } catch (error) {
      if (error instanceof ApiError) {
        const next: State = { kind: "error", message: error.message };
        if (error.requestId) {
          next.requestId = error.requestId;
        }
        setState(next);
      } else {
        setState({
          kind: "error",
          message: error instanceof Error ? error.message : String(error),
        });
      }
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  return (
    <main className="page">
      <header className="header">
        <h1>TigerBeetle Platform</h1>
        <p className="muted">
          Console, pre-alpha. Status from GET /v1/_info, refreshed every 5 seconds.
        </p>
      </header>
      <StatusPanel state={state} onRefresh={refresh} />
    </main>
  );
}

function StatusPanel({ state, onRefresh }: { state: State; onRefresh: () => void }) {
  if (state.kind === "loading") {
    return <p className="muted">Loading service information...</p>;
  }
  if (state.kind === "error") {
    return (
      <section className="card error">
        <h2>Request failed</h2>
        <p>{state.message}</p>
        {state.requestId ? <p className="muted">Request ID: {state.requestId}</p> : null}
        <button type="button" onClick={onRefresh}>
          Retry
        </button>
      </section>
    );
  }
  const { info, fetchedAt } = state;
  return (
    <section className="card">
      <dl className="facts">
        <dt>Service</dt>
        <dd>{info.service}</dd>
        <dt>Version</dt>
        <dd>{info.version}</dd>
        <dt>Last checked</dt>
        <dd>{fetchedAt.toLocaleTimeString()}</dd>
      </dl>
      <h2>Dependencies</h2>
      <ul className="dependencies">
        <Dependency name="Postgres" check={info.dependencies.postgres} />
        <Dependency name="TigerBeetle" check={info.dependencies.tigerbeetle} />
      </ul>
      <button type="button" onClick={onRefresh}>
        Refresh now
      </button>
    </section>
  );
}

function Dependency({ name, check }: { name: string; check: DependencyCheck }) {
  const up = check.status === "up";
  return (
    <li className={up ? "dependency up" : "dependency down"}>
      <span className="indicator" aria-hidden="true" />
      <span className="name">{name}</span>
      <span className="status">{up ? "up" : `down (${check.error ?? "unknown"})`}</span>
      <span className="latency">{check.latencyMs} ms</span>
    </li>
  );
}
