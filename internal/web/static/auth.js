(() => {
  let initiator = null;
  const root = () => document.querySelector('#auth-surface-root');
  const close = () => {
    const container = root();
    if (!container?.querySelector('[data-auth-overlay]')) return;
    if (container.hasAttribute('data-auth-full-page')) {
      window.location.assign('/');
      return;
    }
    container.replaceChildren();
    document.querySelector('.app-frame')?.removeAttribute('inert');
    initiator?.focus();
  };
  const activate = container => {
    const overlay = container?.querySelector?.('[data-auth-overlay]');
    if (!overlay) return;
    document.querySelector('.app-frame')?.setAttribute('inert', '');
    const error = overlay.querySelector('[role="alert"]');
    const field = overlay.querySelector('input:not([type="hidden"])') || overlay.querySelector('button, a');
    (error || field)?.focus();
  };
  document.addEventListener('htmx:beforeRequest', event => {
    if (event.detail?.target?.id === 'auth-surface-root') initiator = document.activeElement;
  });
  document.addEventListener('htmx:afterSwap', event => {
    if (event.target?.id === 'auth-surface-root') activate(event.target);
  });
  document.addEventListener('click', event => {
    const overlay = event.target.closest?.('[data-auth-overlay]');
    if (!overlay) return;
    if (event.target.closest('.floating-close') || event.target === overlay) {
      event.preventDefault();
      close();
    }
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && root()?.querySelector('[data-auth-overlay]')) {
      event.preventDefault();
      close();
    }
  });

  const initialize = root => {
    const form = root.querySelector('[data-sign-in-form]');
    if (!form || form.dataset.rememberInitialized) return;
    form.dataset.rememberInitialized = 'true';
    const username = form.querySelector('[data-remember-username]');
    const remember = form.querySelector('[data-remember-toggle]');
    if (!username || !remember) return;
    const key = 'squashRememberedUsername';
    try {
      const stored = window.localStorage.getItem(key);
      if (!username.value && stored) username.value = stored;
      remember.checked = Boolean(stored && username.value === stored);
    } catch (_) {
      remember.disabled = true;
    }
    form.addEventListener('submit', () => {
      try {
        if (remember.checked) {
          window.localStorage.setItem(key, username.value.trim());
        } else {
          window.localStorage.removeItem(key);
        }
      } catch (_) {
        // Sign-in remains fully functional when browser storage is unavailable.
      }
    });
  };
  initialize(document);
  activate(document);
  document.addEventListener('htmx:afterSwap', event => initialize(event.target));
})();
