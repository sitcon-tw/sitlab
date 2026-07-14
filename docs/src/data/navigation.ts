import { getCollection } from "astro:content";

const sectionOrder = new Map([
	["Start", 0],
	["Guides", 10],
	["Reference", 20]
]);

export function hrefForDocId(id: string) {
	const slug = id.replace(/\.(md|mdx)$/, "").replace(/\/index$/, "");
	return slug === "index" ? "/docs/" : `/docs/${slug}/`;
}

export async function getNavigation() {
	const entries = await getCollection("docs", (entry) => !entry.data.draft);
	return entries
		.map((entry) => ({
			title: entry.data.title,
			description: entry.data.description,
			section: entry.data.section,
			order: entry.data.order,
			href: hrefForDocId(entry.id)
		}))
		.sort((left, right) => {
			const sectionDelta = (sectionOrder.get(left.section) ?? 100) - (sectionOrder.get(right.section) ?? 100);
			return sectionDelta || left.order - right.order || left.title.localeCompare(right.title);
		});
}
