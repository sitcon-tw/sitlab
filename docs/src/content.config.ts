import { glob } from "astro/loaders";
import { z } from "astro/zod";
import { defineCollection } from "astro:content";

const docs = defineCollection({
	loader: glob({ pattern: "**/*.{md,mdx}", base: "./src/content/docs" }),
	schema: z.object({
		title: z.string(),
		description: z.string(),
		section: z.enum(["Start", "Guides", "Reference"]),
		order: z.number(),
		draft: z.boolean().default(false)
	})
});

export const collections = { docs };
