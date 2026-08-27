import { useCallback, useRef, useState, type PointerEvent, type ReactNode } from "react";

interface Ripple {
	id: number;
	size: number;
	x: number;
	y: number;
}

export interface RippleBinding {
	onPointerDown: (event: PointerEvent<HTMLElement>) => void;
	rippleNodes: ReactNode;
}

/**
 * Material Design 3 ripple.
 *
 * React owns the ripple nodes rather than a host.appendChild, so there is no
 * reconciliation race when the host re-renders mid-animation. Each ripple
 * removes itself on animationend.
 *
 * The host must carry the `md-state-layer` class (position, isolation, and the
 * overflow clip the ripple expands inside).
 */
export function useRipple(): RippleBinding {
	const [ripples, setRipples] = useState<Ripple[]>([]);
	const nextId = useRef(0);

	const onPointerDown = useCallback((event: PointerEvent<HTMLElement>) => {
		if (event.button !== 0) return;
		// Bail before touching state so reduced motion costs nothing.
		if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;
		const rect = event.currentTarget.getBoundingClientRect();
		const x = event.clientX - rect.left;
		const y = event.clientY - rect.top;
		const radius = Math.hypot(Math.max(x, rect.width - x), Math.max(y, rect.height - y));
		const id = nextId.current++;
		setRipples((current) => [...current, { id, size: radius * 2, x: x - radius, y: y - radius }]);
	}, []);

	const end = useCallback((id: number) => {
		setRipples((current) => current.filter((ripple) => ripple.id !== id));
	}, []);

	const rippleNodes = ripples.map((ripple) => (
		<span
			key={ripple.id}
			className="md-ripple"
			aria-hidden="true"
			style={{ width: ripple.size, height: ripple.size, left: ripple.x, top: ripple.y }}
			onAnimationEnd={() => end(ripple.id)}
		/>
	));

	return { onPointerDown, rippleNodes };
}
