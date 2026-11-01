import {useSearchParams} from 'react-router-dom';
import {RealBOMWorkspace} from './RealBOMWorkspace';
import {EngineeringWorkbench} from './EngineeringWorkbench';
export function BOMWorkspace(){const [params]=useSearchParams();return params.get('legacy')==='1'?<RealBOMWorkspace/>:<EngineeringWorkbench/>}
