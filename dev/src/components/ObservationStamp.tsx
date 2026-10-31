import type {Row} from '../lib/api';
import {observationMeta} from '../lib/api/rackDetails';
import {Badge} from './ui';
export function ObservationStamp({row}:{row?:Row}){const m=observationMeta(row);return <div className="observation-bar"><span>{m.source}</span><time>{m.observed_at||'采集时间 unknown'}</time><Badge value={m.freshness}/><Badge value={m.support}/></div>}
