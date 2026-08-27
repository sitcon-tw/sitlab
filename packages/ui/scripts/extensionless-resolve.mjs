// @material/material-color-utilities ships ESM with extensionless relative
// imports, which native Node ESM refuses to resolve. This hook retries a failed
// specifier with ".js" and "/index.js" appended. Build-time only.
export async function resolve(specifier, context, nextResolve) {
	try {
		return await nextResolve(specifier, context);
	} catch (error) {
		if (!specifier.startsWith(".") && !specifier.startsWith("/")) throw error;
		for (const suffix of [".js", "/index.js"]) {
			try {
				return await nextResolve(specifier + suffix, context);
			} catch {
				/* try the next candidate */
			}
		}
		throw error;
	}
}
