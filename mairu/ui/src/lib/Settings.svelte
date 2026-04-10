<script lang="ts">
  import { Settings as SettingsIcon, Save, Key, Cpu, ShieldAlert, Bot } from 'lucide-svelte';
</script>

<div class="flex-1 flex flex-col bg-green-950 border-r border-green-900">
  <div class="h-14 border-b border-green-900 flex items-center px-4 justify-between shrink-0 bg-green-950/50 backdrop-blur-sm z-10 sticky top-0">
    <h1 class="text-sm font-semibold tracking-wide flex items-center gap-2">
      <SettingsIcon size={16} />
      Agent Settings
    </h1>
    <button class="flex items-center gap-2 px-3 py-1.5 bg-green-600 hover:bg-green-500 text-black  text-sm font-medium transition-colors shadow-sm">
      <Save size={14} />
      Save Changes
    </button>
  </div>
  
  <div class="flex-1 overflow-y-auto p-6">
    <div class="max-w-2xl mx-auto space-y-8">
      
      <!-- LLM Configuration -->
      <section>
        <h2 class="text-lg font-semibold text-green-400 mb-4 flex items-center gap-2 border-b border-green-900 pb-2">
          <Cpu size={18} class="text-green-400" />
          Model Configuration
        </h2>
        
        <div class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-green-600" for="provider">Primary Provider</label>
              <select id="provider" class="w-full bg-black border border-green-900  px-3 py-2 text-sm text-green-400 focus:outline-none focus:border-green-500 transition-colors">
                <option value="gemini">Google Gemini</option>
                <option value="anthropic">Anthropic Claude</option>
                <option value="openai">OpenAI</option>
                <option value="local">Local (Ollama/LM Studio)</option>
              </select>
            </div>
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-green-600" for="model">Model Name</label>
              <select id="model" class="w-full bg-black border border-green-900  px-3 py-2 text-sm text-green-400 focus:outline-none focus:border-green-500 transition-colors">
                <option value="gemini-2.5-pro">Gemini 2.5 Pro</option>
                <option value="gemini-2.5-flash">Gemini 2.5 Flash</option>
              </select>
            </div>
          </div>
          
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-green-600 flex items-center gap-2">
              <Key size={14} /> API Key
            </label>
            <div class="flex gap-2">
              <input type="password" value="************************" class="flex-1 bg-black border border-green-900  px-3 py-2 text-sm text-green-400 focus:outline-none focus:border-green-500 transition-colors" />
            </div>
            <p class="text-xs text-green-600 mt-1">Leave blank to use environment variables (e.g. GEMINI_API_KEY).</p>
          </div>
        </div>
      </section>

      <!-- System Prompt -->
      <section>
        <h2 class="text-lg font-semibold text-green-400 mb-4 flex items-center gap-2 border-b border-green-900 pb-2">
          <Bot size={18} class="text-emerald-400" />
          Agent Identity
        </h2>
        
        <div class="space-y-1.5">
          <label class="text-sm font-medium text-green-600" for="system-instructions">Custom System Instructions</label>
          <textarea id="system-instructions" 
            rows="4" 
            class="w-full bg-black border border-green-900  px-3 py-2 text-sm text-green-400 focus:outline-none focus:border-green-500 transition-colors resize-none font-mono text-xs"
            placeholder="Additional instructions for the agent..."
          >You are Mairu, an expert software engineering assistant.
You specialize in navigating the contextfs workspace.</textarea>
          <p class="text-xs text-green-600 mt-1">These instructions are appended to the base system prompt.</p>
        </div>
      </section>

      <!-- Security Settings -->
      <section>
        <h2 class="text-lg font-semibold text-green-400 mb-4 flex items-center gap-2 border-b border-green-900 pb-2">
          <ShieldAlert size={18} class="text-amber-400" />
          Security & Permissions
        </h2>
        
        <div class="space-y-3 bg-black border border-green-900  p-4">
          <label class="flex items-start gap-3 cursor-pointer">
            <div class="relative flex items-center mt-0.5">
              <input type="checkbox" checked class="peer sr-only" />
              <div class="w-9 h-5 bg-green-800 peer-focus:outline-none  peer peer-checked:after:translate-x-full peer-checked:after:border-black after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-black after:border-green-900 after:border after: after:h-4 after:w-4 after:transition-all peer-checked:bg-green-500"></div>
            </div>
            <div>
              <div class="text-sm font-medium text-green-400">Require approval for bash commands</div>
              <div class="text-xs text-green-600">Agent will pause and ask before executing any terminal command.</div>
            </div>
          </label>
          
          <label class="flex items-start gap-3 cursor-pointer">
            <div class="relative flex items-center mt-0.5">
              <input type="checkbox" checked class="peer sr-only" />
              <div class="w-9 h-5 bg-green-800 peer-focus:outline-none  peer peer-checked:after:translate-x-full peer-checked:after:border-black after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-black after:border-green-900 after:border after: after:h-4 after:w-4 after:transition-all peer-checked:bg-green-500"></div>
            </div>
            <div>
              <div class="text-sm font-medium text-green-400">Require approval for file modifications</div>
              <div class="text-xs text-green-600">Agent will pause and ask before writing or editing files.</div>
            </div>
          </label>
        </div>
      </section>

    </div>
  </div>
</div>
