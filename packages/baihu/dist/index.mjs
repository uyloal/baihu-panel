import http from 'node:http';
import https from 'node:https';
import { URL } from 'node:url';

function request(urlStr, method = 'GET', data = null) {
  const token = process.env.BHPKG_OPENAPI_TOKEN || process.env.OPENAPI_TOKEN || process.env.BHPKG_NOTIFY_TOKEN;
  if (!token) {
    throw new Error('没有正确配置或缺少 BHPKG_OPENAPI_TOKEN 环境变量。请在白虎面板设置中配置 OpenAPI Token。');
  }

  const parsedUrl = new URL(urlStr);
  const protocol = parsedUrl.protocol === 'https:' ? https : http;

  let payload = '';
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  };

  if (data !== null) {
    payload = JSON.stringify(data);
    headers['Content-Length'] = Buffer.byteLength(payload);
  }

  const options = {
    hostname: parsedUrl.hostname,
    port: parsedUrl.port || (parsedUrl.protocol === 'https:' ? 443 : 80),
    path: parsedUrl.pathname + (parsedUrl.search || ''),
    method: method,
    headers: headers
  };

  return new Promise((resolve, reject) => {
    const req = protocol.request(options, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => (body += chunk));
      res.on('end', () => {
        if (res.statusCode && res.statusCode >= 200 && res.statusCode < 300) {
          try {
            const parsed = body ? JSON.parse(body) : {};
            if (parsed && typeof parsed === 'object' && parsed.code !== undefined && parsed.code !== 200) {
              reject(new Error(`请求失败 [${parsed.code}]: ${parsed.msg || parsed.message || '未知错误'}`));
            } else {
              resolve(parsed.data !== undefined ? parsed.data : parsed);
            }
          } catch {
            resolve(body);
          }
        } else {
          let errMsg = body;
          try {
            const parsed = JSON.parse(body);
            errMsg = parsed.msg || parsed.message || body;
          } catch {}
          reject(new Error(`请求失败 [${res.statusCode}]: ${errMsg}`));
        }
      });
    });

    req.on('error', (e) => reject(e));
    if (payload) {
      req.write(payload);
    }
    req.end();
  });
}

function getEnvsUrl() {
  const url = process.env.BHPKG_OPENAPI_URL || process.env.OPENAPI_URL;
  if (url) return url.endsWith('/env') ? url : `${url}/env`;

  const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
  const targets = ['/api/v1/notify/send/', '/api/v1/notify/send', '/api/v1/notify/', '/api/v1/notify'];
  for (const target of targets) {
    if (notifyUrl.includes(target)) {
      return notifyUrl.replace(target, '/open2api/v1/env');
    }
  }
  return 'http://127.0.0.1:8052/open2api/v1/env';
}

function getBaseUrl() {
  const url = process.env.BHPKG_OPENAPI_URL || process.env.OPENAPI_URL;
  if (url) {
    if (url.endsWith('/env')) return url.slice(0, -4);
    if (url.endsWith('/env/')) return url.slice(0, -5);
    return url;
  }

  const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
  const targets = ['/api/v1/notify/send/', '/api/v1/notify/send', '/api/v1/notify/', '/api/v1/notify'];
  for (const target of targets) {
    if (notifyUrl.includes(target)) {
      return notifyUrl.replace(target, '/open2api/v1');
    }
  }
  return 'http://127.0.0.1:8052/open2api/v1';
}

export function notify(title, content, options) {
  return new Promise((resolve) => {
    let format = 'text';
    let channel_id = '';

    if (typeof options === 'string') {
      if (['text', 'markdown', 'html'].includes(options)) {
        format = options;
      } else {
        channel_id = options;
      }
    } else if (typeof options === 'object' && options !== null) {
      format = options.format || 'text';
      channel_id = options.channel_id || options.channelId || '';
    }

    const token = process.env.BHPKG_NOTIFY_TOKEN || process.env.OPENAPI_TOKEN || process.env.BHPKG_OPENAPI_TOKEN;
    const defaultChannel = process.env.BHPKG_NOTIFY_CHANNEL || '';
    const cid = channel_id || defaultChannel;

    if (!token) {
      resolve(false);
      return;
    }

    const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
    let parsedUrl;
    try {
      parsedUrl = new URL(notifyUrl);
    } catch {
      resolve(false);
      return;
    }

    const client = parsedUrl.protocol === 'https:' ? https : http;
    const postData = JSON.stringify({
      channel_id: cid,
      title: title || '任务运行通知',
      content: content || '',
      format: format
    });

    const req = client.request(
      {
        hostname: parsedUrl.hostname,
        port: parsedUrl.port || (parsedUrl.protocol === 'https:' ? 443 : 80),
        path: parsedUrl.pathname + parsedUrl.search,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'notify-token': token,
          'Authorization': `Bearer ${token}`,
          'Content-Length': Buffer.byteLength(postData)
        },
        timeout: 5000
      },
      (res) => {
        res.resume();
        resolve(res.statusCode !== undefined && res.statusCode >= 200 && res.statusCode < 300);
      }
    );

    req.on('error', () => resolve(false));
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
    req.write(postData);
    req.end();
  });
}

export async function getEnvs() {
  return await request(`${getEnvsUrl()}/all`, 'GET');
}

export async function getEnv(name) {
  const envs = await getEnvs();
  if (Array.isArray(envs)) {
    for (const env of envs) {
      if (env.name === name) return env;
    }
  }
  return null;
}

export async function addEnvs(envsList) {
  const url = getEnvsUrl();
  const addedEnvs = [];
  for (const env of envsList) {
    if (!env.name || env.value === undefined) {
      throw new Error("环境变量必须包含 'name' 和 'value'");
    }
    const res = await request(url, 'POST', env);
    if (res) addedEnvs.push(res);
  }
  return addedEnvs;
}

export async function addEnv(name, value, remark = '', type = 'normal', hidden = true, enabled = true) {
  return await request(getEnvsUrl(), 'POST', { name, value, remark, type, hidden, enabled });
}

export async function updateEnv(id, name, value, remark, type, hidden, enabled) {
  const payload = {};
  if (name !== undefined) payload.name = name;
  if (value !== undefined) payload.value = value;
  if (remark !== undefined) payload.remark = remark;
  if (type !== undefined) payload.type = type;
  if (hidden !== undefined) payload.hidden = hidden;
  if (enabled !== undefined) payload.enabled = enabled;
  return await request(`${getEnvsUrl()}/${id}`, 'PUT', payload);
}

export async function deleteEnvs(ids) {
  for (const id of ids) {
    await deleteEnv(id);
  }
  return true;
}

export async function deleteEnv(id) {
  await request(`${getEnvsUrl()}/${id}`, 'DELETE');
  return true;
}

export async function getTasks() {
  return await request(`${getBaseUrl()}/tasks`, 'GET');
}

export async function getTask(id) {
  return await request(`${getBaseUrl()}/tasks/${id}`, 'GET');
}

export async function updateTask(id, params) {
  return await request(`${getBaseUrl()}/tasks/${id}`, 'PUT', params);
}

export async function deleteTask(id) {
  await request(`${getBaseUrl()}/tasks/${id}`, 'DELETE');
  return true;
}

export async function executeTask(id) {
  return await request(`${getBaseUrl()}/execute/task/${id}`, 'POST');
}

export async function stopTask(logId) {
  return await request(`${getBaseUrl()}/tasks/stop/${logId}`, 'POST');
}

export async function getLastResults() {
  return await request(`${getBaseUrl()}/execute/results`, 'GET');
}

const baihu = {
  notify,
  getEnvs,
  getEnv,
  addEnvs,
  addEnv,
  updateEnv,
  deleteEnvs,
  deleteEnv,
  getTasks,
  getTask,
  updateTask,
  deleteTask,
  executeTask,
  stopTask,
  getLastResults
};

export default baihu;
