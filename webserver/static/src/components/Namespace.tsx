import { useRouteMatch } from 'react-router';
import useFetch from 'use-http';
import { Waybill } from '../lib/spec';
import Result from './Result'

const Namespace: React.FC = () => {
  const doc = useRouteMatch() as {
    params: {
      namespace: string;
    }
  }

  const { data } = useFetch<{
    Waybill: Waybill;
    DiffURLFormat: string;
  }>(`http://localhost:8080/api/v1/status/${doc?.params?.namespace}`, [])

  return (
    <div className="flex flex-col">
      <Result
        expanded
        diffURL={data?.DiffURLFormat}
        status={data?.Waybill.status}
        spec={data?.Waybill.spec}
        metadata={data?.Waybill.metadata}
      />
    </div>
  )
}

export default Namespace
