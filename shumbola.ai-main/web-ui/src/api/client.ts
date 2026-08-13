import type {
  Engagement,
  PendingApproval,
  ReportLang,
  TaskTreeNode,
} from './types';

const API_KEY_HEADER = 'X-API-Key';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export interface Credentials {
  baseUrl: string;
  apiKey: string;
}

async function request<T>(
  creds: Credentials,
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${creds.baseUrl.replace(/\/$/, '')}${path}`, {
    ...init,
    headers: {
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      [API_KEY_HEADER]: creds.apiKey,
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new ApiError(res.status, text || `HTTP ${res.status}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function listEngagements(creds: Credentials): Promise<Engagement[]> {
  return request(creds, '/engagements');
}

export function getEngagement(
  creds: Credentials,
  id: string,
): Promise<Engagement> {
  return request(creds, `/engagements/${encodeURIComponent(id)}`);
}

export function createEngagement(
  creds: Credentials,
  id: string,
  targets: string[],
): Promise<Engagement> {
  return request(creds, '/engagements', {
    method: 'POST',
    body: JSON.stringify({ id, targets }),
  });
}

export function startEngagement(
  creds: Credentials,
  id: string,
): Promise<{ status: string }> {
  return request(creds, `/engagements/${encodeURIComponent(id)}/start`, {
    method: 'POST',
  });
}

export function stopEngagement(
  creds: Credentials,
  id: string,
): Promise<{ stopped: boolean }> {
  return request(creds, `/engagements/${encodeURIComponent(id)}/stop`, {
    method: 'POST',
  });
}

export function getTaskTree(
  creds: Credentials,
  id: string,
): Promise<TaskTreeNode[]> {
  return request(creds, `/engagements/${encodeURIComponent(id)}/tasktree`);
}

export function listApprovals(
  creds: Credentials,
  id: string,
): Promise<PendingApproval[]> {
  return request(creds, `/engagements/${encodeURIComponent(id)}/approvals`);
}

export function decideApproval(
  creds: Credentials,
  engagementId: string,
  approvalId: string,
  approved: boolean,
): Promise<{ approved: boolean }> {
  return request(
    creds,
    `/engagements/${encodeURIComponent(engagementId)}/approvals/${encodeURIComponent(approvalId)}`,
    { method: 'POST', body: JSON.stringify({ approved }) },
  );
}

// downloadReport fetches the PDF as a blob and triggers a browser
// download. A plain <a href> can't be used here — the request needs the
// X-API-Key header, which only JS-driven fetch can attach.
export async function downloadReport(
  creds: Credentials,
  id: string,
  lang: ReportLang,
): Promise<void> {
  const res = await fetch(
    `${creds.baseUrl.replace(/\/$/, '')}/engagements/${encodeURIComponent(id)}/report?lang=${encodeURIComponent(lang)}`,
    { headers: { [API_KEY_HEADER]: creds.apiKey } },
  );
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new ApiError(res.status, text || `HTTP ${res.status}`);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${id}-report.pdf`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
