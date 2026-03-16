import { redirect } from "next/navigation";

type ProjectPageProps = {
  params: Promise<{ id: string }>;
};

export default async function ProjectPage({ params }: ProjectPageProps) {
  const { id } = await params;
  redirect(`/projects/${id}/github`);
}
