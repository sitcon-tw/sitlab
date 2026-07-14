import { useState } from "react";
import styles from "./Avatar.module.css";

export interface AvatarProps {
	name: string;
	src?: string | null;
	size?: "sm" | "md" | "lg";
	className?: string;
}

export function Avatar({ name, src, size = "md", className }: AvatarProps) {
	const [loaded, setLoaded] = useState(false);
	const [failed, setFailed] = useState(false);
	const initial = Array.from(name.trim())[0]?.toUpperCase() ?? "?";
	return (
		<span className={[styles.avatar, styles[size], className].filter(Boolean).join(" ")} aria-hidden="true">
			<span>{initial}</span>
			{src && !failed ? (
				<img src={src} alt="" className={loaded ? styles.loaded : undefined} onLoad={() => setLoaded(true)} onError={() => setFailed(true)} />
			) : null}
		</span>
	);
}
