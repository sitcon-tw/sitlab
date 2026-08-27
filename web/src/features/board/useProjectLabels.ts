import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useMemo } from "react";
import { listProjectLabels } from "./labelsApi";
import type { ProjectLabel } from "./model";

export const projectLabelsQueryKey = ["sitcon", "project-labels"] as const;

/**
 * The GitLab project label catalog.
 *
 * Every consumer goes through this hook so a single invalidation refreshes the
 * filter dialog, the quick-create picker, the drawer picker, and the card chips
 * at once. Labels are read on demand and deliberately stay out of bootstrap.
 */
export function useProjectLabels(): UseQueryResult<ProjectLabel[]> {
	return useQuery({ queryKey: projectLabelsQueryKey, queryFn: listProjectLabels, staleTime: 5 * 60_000 });
}

/** Name to metadata, for rendering label colors away from the fetch site. */
export function useProjectLabelMap(): Map<string, ProjectLabel> {
	const { data } = useProjectLabels();
	return useMemo(() => new Map((data ?? []).map((label) => [label.name, label])), [data]);
}
