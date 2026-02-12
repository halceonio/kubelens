export type UnauthorizedEventDetail = {
  source?: string;
  status?: number;
};

const UNAUTHORIZED_EVENT_NAME = 'kubelens:unauthorized';

let unauthorizedDispatched = false;

export const emitUnauthorized = (detail: UnauthorizedEventDetail = {}) => {
  if (typeof window === 'undefined') return;
  if (unauthorizedDispatched) return;
  unauthorizedDispatched = true;
  window.dispatchEvent(new CustomEvent<UnauthorizedEventDetail>(UNAUTHORIZED_EVENT_NAME, { detail }));
};

export const onUnauthorized = (handler: (detail: UnauthorizedEventDetail) => void) => {
  if (typeof window === 'undefined') {
    return () => {};
  }

  const listener: EventListener = (event) => {
    const customEvent = event as CustomEvent<UnauthorizedEventDetail>;
    handler(customEvent.detail || {});
  };

  window.addEventListener(UNAUTHORIZED_EVENT_NAME, listener);
  return () => window.removeEventListener(UNAUTHORIZED_EVENT_NAME, listener);
};

export const resetUnauthorizedState = () => {
  unauthorizedDispatched = false;
};
