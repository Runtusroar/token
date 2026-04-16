import client from './client';

export const userAPI = {
  getProfile: () => client.get('/user/profile'),

  changePassword: (oldPassword: string, newPassword: string) =>
    client.put('/user/password', { old_password: oldPassword, new_password: newPassword }),

  getDashboard: () => client.get('/user/dashboard'),

  listApiKeys: () => client.get('/user/api-keys'),

  createApiKey: (name: string) => client.post('/user/api-keys', { name }),

  deleteApiKey: (id: number) => client.delete(`/user/api-keys/${id}`),

  updateApiKey: (id: number, data: Record<string, unknown>) =>
    client.put(`/user/api-keys/${id}`, data),

  listLogs: (page: number, pageSize: number) =>
    client.get('/user/logs', { params: { page, page_size: pageSize } }),

  listBalanceLogs: (page: number, pageSize: number) =>
    client.get('/user/balance-logs', { params: { page, page_size: pageSize } }),

  redeem: (code: string) => client.post('/user/redeem', { code }),

  getDailyStats: (days = 7) =>
    client.get('/user/daily-stats', { params: { days } }),

  listModels: () => client.get('/user/models'),
};
