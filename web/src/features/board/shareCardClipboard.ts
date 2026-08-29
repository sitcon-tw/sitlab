/**
 * Delivers the share PNG: clipboard first, file download as the fallback.
 *
 * The ClipboardItem is constructed synchronously inside the user-gesture task
 * and wraps the pending Blob promise — the only pattern Safari accepts for
 * async image producers. Callers must therefore invoke deliverSharePng
 * directly from the click handler without awaiting anything first.
 */

export type ShareDelivery = "copied" | "downloaded";

export function canCopyImageToClipboard(): boolean {
	if (typeof ClipboardItem === "undefined" || typeof navigator.clipboard?.write !== "function") return false;
	// supports() is newer than ClipboardItem itself; absence means PNG works.
	const supports = (ClipboardItem as { supports?: (type: string) => boolean }).supports;
	return supports ? supports.call(ClipboardItem, "image/png") : true;
}

export async function deliverSharePng(getBlob: () => Promise<Blob>, filename: string): Promise<ShareDelivery> {
	if (canCopyImageToClipboard()) {
		const blobPromise = getBlob();
		try {
			await navigator.clipboard.write([new ClipboardItem({ "image/png": blobPromise })]);
			return "copied";
		} catch {
			// Permission denied or the document lost focus mid-write. Re-await the
			// blob so a render failure still rejects instead of downloading nothing.
			return downloadBlob(await blobPromise, filename);
		}
	}
	return downloadBlob(await getBlob(), filename);
}

function downloadBlob(blob: Blob, filename: string): ShareDelivery {
	const url = URL.createObjectURL(blob);
	const anchor = document.createElement("a");
	anchor.href = url;
	anchor.download = filename;
	anchor.rel = "noopener";
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
	// Deferred so the browser has started the download before the URL dies.
	setTimeout(() => URL.revokeObjectURL(url), 10_000);
	return "downloaded";
}
