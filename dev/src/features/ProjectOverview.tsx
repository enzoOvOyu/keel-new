import {Link} from 'react-router-dom';
import {useProject} from '../app/ProjectContext';
import {StageWorkspace} from './StageWorkspace';
import {Badge} from '../components/ui';
export function ProjectOverview(){const {projectId,projects}=useProject();const project=projects.find(p=>p.id===projectId);return <><div className="project-heading"><div><Badge value={project?.code||projectId}/><h2>{project?.name||'项目交付'}</h2><p>接收已审核的开局产物，按实际执行证据推进交付。</p></div><Link className="btn" to={'/p/'+encodeURIComponent(projectId)+'/kickoff/overview'}>智能开局</Link></div><StageWorkspace initialStage="KICKOFF"/></>}
