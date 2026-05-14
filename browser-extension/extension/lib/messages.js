export const MESSAGE_TYPES = {
  page_content: {
    required: ['payload.url', 'payload.html', 'payload.timestamp'],
  },
  get_status: { required: [] },
  get_dev_state: { required: [] },
  get_logs: { required: [] },
  search: { required: [] },
  set_api_url: { required: ['url'] },
  force_sync: { required: [] },
  send_to_agent: { required: ['payload.text'] },
  reset_session: { required: [] },
  clear_queue: { required: [] },
  dev_log: { required: [] },
  dev_force_eval: { required: [] },
  execute: {
    required: ['command'],
    commands: [
      'click',
      'fill',
      'highlight',
      'scroll',
      'navigate',
      'get_text',
      'select_text',
      'query',
      'set_storage',
      'show_thought',
      'hide_thought',
      'highlight_thought',
    ],
    commandRequired: {
      click: ['selector'],
      fill: ['selector', 'value'],
      highlight: ['selector'],
      scroll: [],
      navigate: [],
      get_text: [],
      select_text: ['selector'],
      query: [],
      set_storage: ['key', 'value'],
      show_thought: ['text'],
      hide_thought: [],
      highlight_thought: ['selector'],
    },
  },
};

function getPath(obj, path) {
  return path.split('.').reduce((a, k) => (a == null ? a : a[k]), obj);
}

export function validate(msg) {
  if (!msg || typeof msg !== 'object') {
    return { ok: false, error: { code: 'not_object', message: 'message is not an object' } };
  }
  const spec = MESSAGE_TYPES[msg.type];
  if (!spec) {
    return { ok: false, error: { code: 'unknown_type', message: `unknown type ${msg.type}` } };
  }
  for (const f of spec.required) {
    if (getPath(msg, f) === undefined) {
      return { ok: false, error: { code: 'missing_field', field: f, message: `missing ${f}` } };
    }
  }
  if (msg.type === 'execute') {
    if (!spec.commands.includes(msg.command)) {
      return { ok: false, error: { code: 'unknown_command', message: `unknown command ${msg.command}` } };
    }
    for (const f of spec.commandRequired[msg.command] || []) {
      if (getPath(msg, f) === undefined) {
        return { ok: false, error: { code: 'missing_field', field: f, message: `missing ${f}` } };
      }
    }
  }
  return { ok: true };
}
