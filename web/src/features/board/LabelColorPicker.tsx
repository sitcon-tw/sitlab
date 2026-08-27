import type { CSSProperties } from "react";
import { useId } from "react";
import styles from "./LabelColorPicker.module.css";
import { labelPalette, normalizeLabelColor } from "./labelPalette";

/**
 * Swatches render through the --tag-color custom property, the same mechanism
 * TagSwatch uses, so no color literal appears in this file or its stylesheet.
 * The native color input and the hex field cover anything off-palette.
 */
export function LabelColorPicker({ value, onChange, label }: { value: string; onChange: (color: string) => void; label: string }) {
	const id = useId();
	const normalized = normalizeLabelColor(value);
	return (
		<div className={styles.picker} role="group" aria-label={label}>
			<div className={styles.swatches}>
				{labelPalette.map((color) => (
					<button
						key={color}
						type="button"
						className={styles.swatch}
						style={{ "--tag-color": color } as CSSProperties}
						aria-label={`使用顏色 ${color}`}
						title={color}
						aria-pressed={normalized === color}
						onClick={() => onChange(color)}
					/>
				))}
			</div>
			<div className={styles.custom}>
				<input
					type="color"
					className={styles.native}
					aria-label={`${label}：自訂顏色`}
					value={normalized ?? labelPalette[0]}
					onChange={(event) => onChange(event.target.value.toUpperCase())}
				/>
				<label className={styles.hex} htmlFor={id}>
					<span className={styles.srOnly}>{label} 色碼</span>
					<input
						id={id}
						value={value}
						spellCheck={false}
						aria-invalid={normalized ? undefined : true}
						placeholder="#RRGGBB"
						onChange={(event) => onChange(event.target.value)}
					/>
				</label>
			</div>
		</div>
	);
}
