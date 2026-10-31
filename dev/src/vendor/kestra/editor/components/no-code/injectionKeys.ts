// Keel adapter: the two injection keys consumed by the native panel manager.
import type {ComputedRef, InjectionKey, Ref} from 'vue';
import type {Panel} from '../../utils/multiPanelTypes';
export const VISIBLE_PANELS_INJECTION_KEY=Symbol('visible-panels-injection-key') as InjectionKey<Ref<Panel[]>>;
export const PANEL_MAXIMIZED_INJECTION_KEY=Symbol('panel-maximized-injection-key') as InjectionKey<ComputedRef<boolean>>;
