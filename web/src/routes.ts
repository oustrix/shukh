// Единый источник форм URL. /r/CODE — та самая шаримая инвайт-ссылка (D-2, W3-2);
// стадия партии живёт в снапшоте, а не в адресе (W3-1), поэтому маршрут один.
export const ROOM_ROUTE = '/r/:code'
export const roomPath = (code: string) => `/r/${code}`
export const STORED_NAME_KEY = 'shukh.name'
