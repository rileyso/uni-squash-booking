(() => {
  const form = document.querySelector('[data-attendance-form]');
  if (!form) return;
  const start = form.querySelector('[data-start]');
  const end = form.querySelector('[data-end]');
  const startRange = form.querySelector('[data-start-range]');
  const endRange = form.querySelector('[data-end-range]');
  const startLabel = form.querySelector('[data-start-label]');
  const endLabel = form.querySelector('[data-end-label]');
  const summary = form.querySelector('[data-summary]');
  const rangeControl = form.querySelector('.single-range-control');
  const rangeFill = form.querySelector('[data-range-fill]');
  const submit = form.querySelector('button[type="submit"]');
  if (!start || !end || !startRange || !endRange || !summary) return;
  const sliderMin = Number(startRange.min || 600);
  const sliderMax = Number(startRange.max || 1320);
  const minDuration = Number(form.dataset.minDuration || 60);
  const step = Number(startRange.step || 30);
  const ranges = (form.dataset.openRanges || '')
    .split(',')
    .map(item => item.split('-').map(value => Number(value)))
    .filter(([a, b]) => Number.isFinite(a) && Number.isFinite(b) && b - a >= minDuration)
    .map(([a, b]) => ({start: a, end: b}));
  const clamp = (value, low, high) => Math.min(high, Math.max(low, value));
  const snap = value => clamp(Math.round(value / step) * step, sliderMin, sliderMax);
  const includesInterval = (range, a, b) => range && a >= range.start && b <= range.end && b - a >= minDuration;
  const rangeForInterval = (a, b) => ranges.find(range => includesInterval(range, a, b));
  const rangeForStart = value => {
    const eligible = ranges.find(range => value >= range.start && value <= range.end - minDuration);
    if (eligible) return eligible;
    return ranges.reduce((best, range) => {
      const candidate = clamp(value, range.start, range.end - minDuration);
      const distance = Math.abs(candidate - value);
      return !best || distance < best.distance ? {range, distance} : best;
    }, null)?.range;
  };
  const rangeForEnd = value => {
    const eligible = ranges.find(range => value >= range.start + minDuration && value <= range.end);
    if (eligible) return eligible;
    return ranges.reduce((best, range) => {
      const candidate = clamp(value, range.start + minDuration, range.end);
      const distance = Math.abs(candidate - value);
      return !best || distance < best.distance ? {range, distance} : best;
    }, null)?.range;
  };
  const label = minutes => {
    if (minutes === 1440) return '12:00 am';
    const hour = Math.floor(minutes / 60);
    const minute = minutes % 60;
    return new Date(2000, 0, 1, hour, minute).toLocaleTimeString([], {hour:'numeric', minute:'2-digit'});
  };
  const setValues = (a, b) => {
    start.value = String(a);
    end.value = String(b);
    startRange.value = String(a);
    endRange.value = String(b);
  };
  const updateFill = (a, b) => {
    if (!rangeFill) return;
    const span = sliderMax - sliderMin;
    rangeFill.style.left = `${((a - sliderMin) / span) * 100}%`;
    rangeFill.style.width = `${((b - a) / span) * 100}%`;
  };
  const minuteFromPointer = event => {
    const rect = rangeControl.getBoundingClientRect();
    const position = clamp((event.clientX - rect.left) / rect.width, 0, 1);
    return snap(sliderMin + position * (sliderMax - sliderMin));
  };
  const nearestHandle = value => {
    const a = Number(start.value);
    const b = Number(end.value);
    return Math.abs(value - a) <= Math.abs(value - b) ? startRange : endRange;
  };
  const moveHandle = (handle, value) => {
    handle.value = String(value);
    if (handle === startRange) start.value = String(value);
    if (handle === endRange) end.value = String(value);
    sync(handle);
  };
  const applyValidInterval = source => {
    let a = snap(Number(start.value || startRange.value));
    let b = snap(Number(end.value || endRange.value));
    if (source === startRange) a = snap(Number(startRange.value));
    if (source === endRange) b = snap(Number(endRange.value));
    if (source === start) a = snap(Number(start.value));
    if (source === end) b = snap(Number(end.value));

    let selectedRange = rangeForInterval(a, b);
    if (!selectedRange && (source === endRange || source === end)) {
      selectedRange = rangeForEnd(b);
      b = clamp(b, selectedRange.start + minDuration, selectedRange.end);
      a = clamp(a, selectedRange.start, b - minDuration);
    } else if (!selectedRange) {
      selectedRange = rangeForStart(a);
      a = clamp(a, selectedRange.start, selectedRange.end - minDuration);
      b = clamp(b, a + minDuration, selectedRange.end);
    }
    if (b - a < minDuration) {
      if (source === endRange || source === end) {
        a = b - minDuration;
      } else {
        b = a + minDuration;
      }
    }
    a = clamp(a, selectedRange.start, selectedRange.end - minDuration);
    b = clamp(b, a + minDuration, selectedRange.end);
    setValues(a, b);
    return [a, b];
  };
  const sync = source => {
    if (!ranges.length) {
      startRange.disabled = true;
      endRange.disabled = true;
      if (submit) submit.disabled = true;
      summary.textContent = 'No continuous open-play interval is available for this date.';
      return;
    }
    const [a, b] = applyValidInterval(source);
    const duration = b - a;
    const hours = duration / 60;
    const durationLabel = duration === 30 ? '30 minutes' : `${hours % 1 ? hours.toFixed(1) : hours} ${hours === 1 ? 'hour' : 'hours'}`;
    if (startLabel) startLabel.textContent = label(a);
    if (endLabel) endLabel.textContent = label(b);
    updateFill(a, b);
    summary.textContent = `${label(a)}–${label(b)} (${durationLabel})`;
  };
  [start,end,startRange,endRange].forEach(control => control.addEventListener('input', () => sync(control)));
  if (rangeControl) {
    let activeHandle = null;
    rangeControl.addEventListener('pointerdown', event => {
      if (!ranges.length || event.button > 0) return;
      event.preventDefault();
      const value = minuteFromPointer(event);
      activeHandle = nearestHandle(value);
      activeHandle.focus();
      rangeControl.setPointerCapture(event.pointerId);
      moveHandle(activeHandle, value);
    });
    rangeControl.addEventListener('pointermove', event => {
      if (!activeHandle) return;
      event.preventDefault();
      moveHandle(activeHandle, minuteFromPointer(event));
    });
    const stopDrag = event => {
      if (!activeHandle) return;
      if (rangeControl.hasPointerCapture(event.pointerId)) {
        rangeControl.releasePointerCapture(event.pointerId);
      }
      activeHandle = null;
    };
    rangeControl.addEventListener('pointerup', stopDrag);
    rangeControl.addEventListener('pointercancel', stopDrag);
  }
  sync();
})();

(() => {
  const cache = new Map();
  const loadNames = async (date, start, end) => {
    const key = `${date}:${start}:${end}`;
    if (!cache.has(key)) {
      const query = new URLSearchParams({date, start: String(start), end: String(end)});
      cache.set(key, fetch(`/attendance/participants?${query}`, {headers: {'Accept': 'application/json'}})
        .then(response => response.ok ? response.json() : {names: []})
        .then(data => Array.isArray(data.names) ? data.names : []).catch(() => []));
    }
    return cache.get(key);
  };
  const renderNames = (container, names) => {
    container.replaceChildren();
    const text = document.createElement('span');
    text.textContent = names.length ? names.join(', ') : 'Empty';
    container.append(text);
  };
  const enhance = root => {
    root.querySelectorAll('.horizontal-calendar').forEach(table => {
      const headers = [...table.querySelectorAll('thead th')];
      table.querySelectorAll('tbody tr:last-child td:has(> .court-status.open)').forEach(cell => {
        if (cell.querySelector('.participant-hover')) return;
        const column = [...cell.parentElement.children].indexOf(cell);
        const label = headers[column]?.textContent.trim();
        const hour = Number(label?.split(':')[0]);
        const dateLink = cell.querySelector('a[href*="date="]');
        if (!Number.isFinite(hour) || !dateLink) return;
        const date = new URL(dateLink.href).searchParams.get('date');
        const box = document.createElement('aside');
        box.className = 'participant-hover';
        box.setAttribute('aria-live', 'polite');
        cell.append(box);
        let loaded = false;
        const show = async () => {
          if (loaded) return;
          loaded = true;
          box.textContent = 'Loading participants…';
          renderNames(box, await loadNames(date, hour * 60, hour * 60 + 60));
        };
        cell.addEventListener('pointerenter', show);
        cell.addEventListener('focusin', show);
      });
    });
    const panel = root.querySelector('.selection-panel');
    const form = panel?.querySelector('form[action="/attendance"]');
    if (panel && form && !panel.querySelector('.selection-participants')) {
      const date = form.querySelector('input[name="date"]')?.value;
      const starts = [...form.querySelectorAll('input[name="range_start"]')];
      const ends = [...form.querySelectorAll('input[name="range_end"]')];
      if (date && starts.length === ends.length) {
        const container = document.createElement('section');
        container.className = 'selection-participants';
        container.textContent = 'Loading participants…';
        form.before(container);
        Promise.all(starts.map((input, index) => loadNames(date, Number(input.value), Number(ends[index].value))))
          .then(groups => renderNames(container, [...new Set(groups.flat())].sort((a, b) => a.localeCompare(b))));
      }
    }
  };
  enhance(document);
  document.addEventListener('htmx:afterSwap', event => enhance(event.target));
})();

(() => {
  const disablePlannedCells = root => {
    root.querySelectorAll('.plan-marker').forEach(marker => {
      const cell = marker.closest('td, li');
      if (!cell) return;
      cell.classList.add('existing-plan-cell');
      cell.querySelectorAll('a[href*="time="]').forEach(link => {
        link.removeAttribute('href');
        link.setAttribute('aria-disabled', 'true');
        link.setAttribute('title', 'Already in your planned attendance');
      });
    });
  };
  disablePlannedCells(document);
  document.addEventListener('htmx:afterSwap', event => disablePlannedCells(event.target));
})();
