import dayjs from 'dayjs';

export const formatLocalDateTime = (date: Date | string | null | undefined): string => {
  if (!date) return '-';
  const parsed = dayjs(date);
  if (!parsed.isValid()) return '-';
  return parsed.format('YYYY-MM-DD HH:mm:ss');
};
