import { useSession } from "@/features/auth/SessionContext";
import { SpinnerLayout } from "@project-template/ui";
import { Navigate, Outlet, useLocation } from "react-router-dom";

export function ProtectedRoute() {
	const { user, loading } = useSession();
	const location = useLocation();
	if (loading) return <SpinnerLayout label="Checking your session" />;
	if (!user) return <Navigate to="/login" state={{ from: `${location.pathname}${location.search}` }} replace />;
	return <Outlet />;
}
