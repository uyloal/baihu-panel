import { notify } from './notify.js';
import {
  getEnvs,
  getEnv,
  addEnvs,
  addEnv,
  updateEnv,
  deleteEnvs,
  deleteEnv
} from './env.js';
import {
  getTasks,
  getTask,
  updateTask,
  deleteTask,
  executeTask,
  stopTask,
  getLastResults
} from './task.js';

export * from './types.js';
export {
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
