import { Component, type ErrorInfo, type ReactNode } from "react";

interface AppErrorBoundaryProps {
	children: ReactNode;
}

interface AppErrorBoundaryState {
	error: Error | null;
}

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
	state: AppErrorBoundaryState = { error: null };

	static getDerivedStateFromError(error: Error): AppErrorBoundaryState {
		return { error };
	}

	componentDidCatch(error: Error, info: ErrorInfo) {
		console.error("sitcon_board_render_failed", { error, componentStack: info.componentStack });
	}

	render() {
		if (!this.state.error) return this.props.children;

		return (
			<main className="sb-startup-error" role="alert">
				<p className="sb-brand">SITCON / 2027</p>
				<h1>看板畫面發生錯誤</h1>
				<p>你的資料沒有遺失。請重新載入看板；如果問題持續發生，請將發生時間提供給維護者。</p>
				<button type="button" className="sb-button sb-button-primary" onClick={() => window.location.reload()}>
					重新載入
				</button>
			</main>
		);
	}
}
