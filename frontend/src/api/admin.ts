import client from './client';

export const adminAPI = {
  getDashboard: () => client.get('/admin/dashboard'),

  listUsers: (page: number, pageSize: number, search?: string) =>
    client.get('/admin/users', { params: { page, page_size: pageSize, search } }),

  updateUser: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/users/${id}`, data),

  topUp: (userId: number, amount: number) =>
    client.post('/admin/users/topup', { user_id: userId, amount }),

  listChannels: () => client.get('/admin/channels'),

  createChannel: (data: Record<string, unknown>) =>
    client.post('/admin/channels', data),

  updateChannel: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/channels/${id}`, data),

  deleteChannel: (id: number) => client.delete(`/admin/channels/${id}`),

  testChannel: (id: number) => client.post(`/admin/channels/${id}/test`),

  listModels: () => client.get('/admin/models'),

  createModel: (data: Record<string, unknown>) =>
    client.post('/admin/models', data),

  updateModel: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/models/${id}`, data),

  listRedeemCodes: () => client.get('/admin/redeem'),

  createRedeemCodes: (data: Record<string, unknown>) =>
    client.post('/admin/redeem', data),

  updateRedeemCode: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/redeem/${id}`, data),

  listLogs: (page: number, pageSize: number, userId?: number, model?: string) =>
    client.get('/admin/logs', { params: { page, page_size: pageSize, user_id: userId, model } }),

  getSettings: () => client.get('/admin/settings'),

  updateSettings: (data: Record<string, unknown>) =>
    client.put('/admin/settings', data),
};
