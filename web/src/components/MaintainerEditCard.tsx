"use client";

import { Card } from "clo-ui/components/Card";
import styles from "./MaintainerEditCard.module.css";

export type MaintainerEditDraft = {
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  location: string;
  status: string;
  companyId: number | null;
};

export type CompanyOption = {
  id: number;
  name: string;
};

type MaintainerEditCardProps = {
  draft: MaintainerEditDraft;
  companies: CompanyOption[];
  isEditing: boolean;
  isDirty: boolean;
  saveStatus: "idle" | "saving";
  saveError: string | null;
  disableName?: boolean;
  disableGitHub?: boolean;
  disableGitHubEmail?: boolean;
  disableLocation?: boolean;
  disableStatus?: boolean;
  disableCompanyAdd?: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onChange: (next: MaintainerEditDraft) => void;
  onSave: () => void;
  onAddCompany: () => void;
};

export default function MaintainerEditCard({
  draft,
  companies,
  isEditing,
  isDirty,
  saveStatus,
  saveError,
  disableName = false,
  disableGitHub = false,
  disableGitHubEmail = false,
  disableLocation = false,
  disableStatus = false,
  disableCompanyAdd = false,
  onEdit,
  onCancel,
  onChange,
  onSave,
  onAddCompany,
}: MaintainerEditCardProps) {
  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.header}>
          <h2 className={styles.title}>Update maintainer</h2>
          {!isEditing ? (
            <button className={styles.editButton} type="button" onClick={onEdit}>
              Edit
            </button>
          ) : (
            <button className={styles.cancelButton} type="button" onClick={onCancel}>
              Cancel
            </button>
          )}
        </div>
        <div className={styles.grid}>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>Name</span>
            <input
              type="text"
              value={draft.name}
              onChange={(event) =>
                onChange({ ...draft, name: event.target.value })
              }
              disabled={!isEditing || disableName}
            />
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>Email</span>
            <input
              type="email"
              value={draft.email}
              onChange={(event) =>
                onChange({ ...draft, email: event.target.value })
              }
              disabled={!isEditing}
            />
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>GitHub account</span>
            <input
              type="text"
              value={draft.github}
              onChange={(event) =>
                onChange({ ...draft, github: event.target.value })
              }
              disabled={!isEditing || disableGitHub}
            />
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>GitHub Email</span>
            <input
              type="email"
              value={draft.githubEmail}
              onChange={(event) =>
                onChange({ ...draft, githubEmail: event.target.value })
              }
              disabled={!isEditing || disableGitHubEmail}
            />
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>Location</span>
            <input
              type="text"
              value={draft.location}
              onChange={(event) =>
                onChange({ ...draft, location: event.target.value })
              }
              disabled={!isEditing || disableLocation}
            />
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>Status</span>
            <select
              value={draft.status}
              onChange={(event) =>
                onChange({ ...draft, status: event.target.value })
              }
              disabled={!isEditing || disableStatus}
            >
              <option value="Active">Active</option>
              <option value="Emeritus">Emeritus</option>
              <option value="Retired">Retired</option>
              <option value="Archived">Archived</option>
            </select>
          </label>
          <label
            className={styles.field}
            title={!isEditing ? "Click Edit to update this record." : undefined}
          >
            <span>Company</span>
            <div className={styles.companyRow}>
              <select
                value={draft.companyId ?? ""}
                onChange={(event) =>
                  onChange({
                    ...draft,
                    companyId: event.target.value ? Number(event.target.value) : null,
                  })
                }
                disabled={!isEditing}
              >
                <option value="">No company</option>
                {companies.map((company) => (
                  <option key={company.id} value={company.id}>
                    {company.name}
                  </option>
                ))}
              </select>
              <button
                className={styles.addCompanyButton}
                type="button"
                onClick={onAddCompany}
                disabled={disableCompanyAdd || !isEditing}
              >
                Change company affiliation
              </button>
            </div>
          </label>
        </div>
        {saveError && <div className={styles.error}>{saveError}</div>}
        <div className={styles.actions}>
          <button
            className={styles.saveButton}
            type="button"
            onClick={onSave}
            disabled={!isEditing || !isDirty || saveStatus === "saving"}
          >
            {saveStatus === "saving" ? "Saving…" : "Save changes"}
          </button>
        </div>
      </div>
    </Card>
  );
}
