function encId(id: string): string {
  return encodeURIComponent(id);
}

export function actionUrl(id: string): string {
  return `/actions/${encId(id)}`;
}

export function actionExecutionsUrl(actionId: string): string {
  return `/actions/${encId(actionId)}/executions`;
}

export function executionUrl(actionId: string, executionId: number): string {
  return `/actions/${encId(actionId)}/executions/${executionId}`;
}
