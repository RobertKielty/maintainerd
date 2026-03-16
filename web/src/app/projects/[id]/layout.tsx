import ProjectRouteClient from "./ProjectRouteClient";

export default function ProjectLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return <ProjectRouteClient>{children}</ProjectRouteClient>;
}
