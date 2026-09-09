import { http } from '../http';
import type {
	StatisticsData,
	AssetOverview,
	AssetPoint
} from '$lib/types';

export const statsApi = {
	overview: (params: any) =>
		http.get<StatisticsData>('/statistics', { params }),
	assets: () => http.get<AssetOverview>('/statistics/assets'),
	timeline: (params?: any) =>
		http.get<AssetPoint[]>('/statistics/assets/timeline', { params })
};
