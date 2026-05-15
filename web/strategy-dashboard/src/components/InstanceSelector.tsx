import type { Instance } from '../api/types';

interface Props {
  instances: Instance[];
  selected: string;
  onSelect: (id: string) => void;
}

export function InstanceSelector({ instances, selected, onSelect }: Props) {
  return (
    <div className="flex items-center gap-3">
      <select
        value={selected}
        onChange={(e) => onSelect(e.target.value)}
        className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
      >
        {instances.map((inst) => (
          <option key={inst.id} value={inst.id}>
            {inst.id} ({inst.type})
          </option>
        ))}
      </select>
      {instances.map((inst) =>
        inst.id === selected ? (
          <div key={inst.id} className="flex items-center gap-2 text-xs">
            <span
              className={`inline-block w-2 h-2 rounded-full ${
                inst.running ? 'bg-green-500' : 'bg-red-500'
              }`}
            />
            <span className={inst.running ? 'text-green-400' : 'text-red-400'}>
              {inst.running ? 'Running' : 'Stopped'}
            </span>
            <span className="text-gray-600">|</span>
            <span className="text-gray-500">Account: {inst.account_id}</span>
          </div>
        ) : null,
      )}
    </div>
  );
}
