const configuredBase = import.meta.env.BASE_URL;
const basePrefix = configuredBase === "/" ? "" : configuredBase.replace(/\/$/, "");

export function withBase(path: `/${string}`) {
	if (!basePrefix || path === basePrefix || path.startsWith(`${basePrefix}/`)) return path;
	return `${basePrefix}${path}`;
}
