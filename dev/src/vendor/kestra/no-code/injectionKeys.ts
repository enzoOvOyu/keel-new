import type {InjectionKey,Ref} from 'vue';
export const BLOCK_VALIDATION_ISSUES_INJECTION_KEY:InjectionKey<Ref<Map<string,string[]>>>=Symbol('block-validation');
export const PANEL_MAXIMIZED_INJECTION_KEY:InjectionKey<Ref<boolean>>=Symbol('panel-maximized');
