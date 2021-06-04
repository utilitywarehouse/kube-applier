import useFetch from 'use-http';
import { Waybill } from '../lib/spec';
import Result from './Result'

const Index: React.FC = () => {
  const { data } = useFetch<{
    Waybills: Waybill[];
    DiffURLFormat: string;
  }>('http://localhost:8080/api/v1/status', [])
  return (
    <div className="">
      <h1 className="text-4xl my-8 font-bold">kube-applier</h1>
      <div className="space-y-2">
        {data?.Waybills?.map(({ spec, status, metadata }, i) => (
          <Result
            key={metadata.namespace+i}
            diffURL={data?.DiffURLFormat}
            status={status}
            spec={spec}
            metadata={metadata}
          />
        ))}
      </div>
    </div>
  )
}

export default Index
