import { Button, EmptyState, PageShell, Panel } from "@project-template/ui";
import { FileQuestion } from "lucide-react";
import { useNavigate } from "react-router-dom";

export function NotFoundPage() {
	const navigate = useNavigate();
	return (
		<PageShell title="Page not found">
			<Panel>
				<EmptyState
					title="Nothing at this address"
					description="The page may have moved, or the link may be incomplete."
					icon={<FileQuestion size="2rem" />}
					action={<Button onClick={() => navigate("/")}>Go to workspace</Button>}
				/>
			</Panel>
		</PageShell>
	);
}
